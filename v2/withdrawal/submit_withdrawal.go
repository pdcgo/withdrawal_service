package withdrawal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math"

	"connectrpc.com/connect"
	"github.com/pdcgo/schema/services/common/v1"
	"github.com/pdcgo/schema/services/revenue_iface/v1"
	withdrawal_iface_v1 "github.com/pdcgo/schema/services/withdrawal_iface/v1"
	"github.com/pdcgo/schema/services/withdrawal_iface/v2"
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/authorization_iface"
	withdrawal_service_v1 "github.com/pdcgo/withdrawal_service"
	"github.com/pdcgo/withdrawal_service/datasource"
	"github.com/pdcgo/withdrawal_service/marketplace_query"
	datasource_v2 "github.com/pdcgo/withdrawal_service/v2/datasource"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SubmitWithdrawal implements withdrawal_ifaceconnect.WithdrawalServiceHandler.
func (w *wdServiceImpl) SubmitWithdrawal(
	ctx context.Context,
	req *connect.Request[withdrawal_iface.SubmitWithdrawalRequest],
	stream *connect.ServerStream[withdrawal_iface.SubmitWithdrawalResponse],
) error {
	var err error

	pay := req.Msg
	// db := w.db.WithContext(ctx)

	streamlog := func(format string, a ...any) error {
		return stream.Send(&withdrawal_iface.SubmitWithdrawalResponse{
			Message: fmt.Sprintf(format, a...),
		})
	}

	identity := w.
		auth.
		AuthIdentityFromHeader(req.Header())

	agent := identity.Identity()

	err = identity.
		Err()

	if err != nil {
		return err
	}
	token := req.Header().Get("Authorization")

	streamlog("sync importer ke versi sebelumnya..")
	reqv1 := connect.NewRequest(&withdrawal_iface_v1.SubmitWithdrawalRequest{
		TeamId:      pay.TeamId,
		MpId:        pay.MpSubmit.MpId,
		Source:      pay.Source,
		MpType:      pay.MpSubmit.MpType,
		ResourceUri: pay.ResourceUri,
	})

	reqv1.Header().Set("Authorization", token)
	_, err = w.v1service.SubmitWithdrawal(ctx, reqv1)

	if err != nil {
		return err
	}

	streamlog("proses updating data accounting..")
	streamlog("membaca file..")
	var importer withdrawal_service_v1.WdImporterIterate
	var data []byte
	data, err = w.storage.GetContent(ctx, pay.ResourceUri)
	if err != nil {
		streamlog("error reading %s", pay.ResourceUri)
		return err
	}

	importer, err = w.createImporter(pay.MpSubmit.MpType, data)
	if err != nil {
		streamlog("error create importer %s", pay.ResourceUri)
		return err
	}

	streamlog("check toko dan marketplace ..")
	var mp *db_models.Marketplace
	mp, err = w.checkShop(
		importer,
		uint(pay.TeamId),
		pay.MpSubmit,
		agent,
	)
	if err != nil {
		streamlog(err.Error())
		return err
	}

	rstream := w.rclient.RevenueStream(ctx)
	rstream.Send(&revenue_iface.RevenueStreamRequest{
		Event: &revenue_iface.RevenueStreamEvent{
			Kind: &revenue_iface.RevenueStreamEvent_Init{
				Init: &revenue_iface.RevenueStreamEventInit{
					Token:  token,
					TeamId: pay.TeamId,
					ShopId: uint64(mp.ID),
					UserId: uint64(agent.IdentityID()),
				},
			},
		},
	})

	err = importer.Iterate(ctx, func(item *db_models.InvoItem) error {
		var err error
		streamlog("processing %s %s at %s", item.Type, item.ExternalOrderID, item.TransactionDate.String())

		if item.Amount == 0 {
			return nil
		}

		switch item.Type {
		case db_models.AdjOrderFund:
			// getting order estimated amount
			var ord *db_models.Order
			ord, err = w.orderRepo.OrderByExternalID(item.ExternalOrderID)
			if err != nil {
				return err
			}

			if ord.ID == 0 {
				streamlog("cannot get order by order id %s", item.ExternalOrderID)
				return nil
			}

			estAmount := float64(ord.OrderMpTotal)
			if estAmount == 0 {
				estAmount = item.Amount
			}

			if item.Amount < 0 {
				msg := &revenue_iface.RevenueStreamRequest{
					Event: &revenue_iface.RevenueStreamEvent{
						Kind: &revenue_iface.RevenueStreamEvent_Fund{
							Fund: &revenue_iface.RevenueStreamEventFund{
								EstAmount: estAmount,
								Amount:    item.Amount,
								At:        timestamppb.New(item.TransactionDate),
								Desc:      fmt.Sprintf("%s on order %s", item.Description, item.ExternalOrderID),
								OrderId:   item.ExternalOrderID,
							},
						},
					},
				}

				// ms2 := &revenue_iface.RevenueStreamRequest{
				// 	Event: &revenue_iface.RevenueStreamEvent{
				// 		Kind: &revenue_iface.RevenueStreamEvent_Adjustment{
				// 			Adjustment: &revenue_iface.RevenueStreamEventAdjustment{
				// 				Amount:  item.Amount,
				// 				At:      timestamppb.New(item.TransactionDate),
				// 				Desc:    fmt.Sprintf("withdrawal negative %s on order %s", item.Description, item.ExternalOrderID),
				// 				OrderId: item.ExternalOrderID,
				// 				Tags: []string{
				// 					"wd_negative",
				// 				},
				// 				Type: revenue_iface.RevenueAdjustmentType_REVENUE_ADJUSTMENT_TYPE_RETURN,
				// 			},
				// 		},
				// 	},
				// }
				err = rstream.Send(msg)

			} else {
				msg := &revenue_iface.RevenueStreamRequest{
					Event: &revenue_iface.RevenueStreamEvent{
						Kind: &revenue_iface.RevenueStreamEvent_Fund{
							Fund: &revenue_iface.RevenueStreamEventFund{
								EstAmount: estAmount,
								Amount:    item.Amount,
								At:        timestamppb.New(item.TransactionDate),
								Desc:      fmt.Sprintf("%s on order %s", item.Description, item.ExternalOrderID),
								OrderId:   item.ExternalOrderID,
							},
						},
					},
				}
				err = rstream.Send(msg)
			}

		case db_models.AdjCommision, db_models.AdjLostCompensation, db_models.AdjCompensation:
			err = rstream.Send(&revenue_iface.RevenueStreamRequest{
				Event: &revenue_iface.RevenueStreamEvent{
					Kind: &revenue_iface.RevenueStreamEvent_Adjustment{
						Adjustment: &revenue_iface.RevenueStreamEventAdjustment{
							Amount:  item.Amount,
							At:      timestamppb.New(item.TransactionDate),
							Desc:    fmt.Sprintf("%s on order %s", item.Description, item.ExternalOrderID),
							OrderId: item.ExternalOrderID,
						},
					},
				},
			})
		case db_models.AdjUnknown, db_models.AdjUnknownAdj:
			err = rstream.Send(&revenue_iface.RevenueStreamRequest{
				Event: &revenue_iface.RevenueStreamEvent{
					Kind: &revenue_iface.RevenueStreamEvent_Adjustment{
						Adjustment: &revenue_iface.RevenueStreamEventAdjustment{
							Amount:  item.Amount,
							At:      timestamppb.New(item.TransactionDate),
							Desc:    fmt.Sprintf("%s on order %s", item.Description, item.ExternalOrderID),
							OrderId: item.ExternalOrderID,
						},
					},
				},
			})
		case db_models.AdjFund:
			err = rstream.Send(&revenue_iface.RevenueStreamRequest{
				Event: &revenue_iface.RevenueStreamEvent{
					Kind: &revenue_iface.RevenueStreamEvent_Withdrawal{
						Withdrawal: &revenue_iface.RevenueStreamEventWithdrawal{
							Amount: math.Abs(item.Amount),
							At:     timestamppb.New(item.TransactionDate),
							Desc:   fmt.Sprintf("[withdrawal] %s", item.Description),
						},
					},
				},
			})
		}

		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		streamlog(err.Error())
		return err
	}

	_, err = rstream.CloseAndReceive()
	return err
}

type xlsSource interface {
	GetShopUsername() (string, error)
}

func (w *wdServiceImpl) checkShop(
	importer xlsSource,
	teamID uint,
	payload *withdrawal_iface.MpSubmit,
	agent authorization_iface.Identity,
) (*db_models.Marketplace, error) {
	var err error
	var mp *db_models.Marketplace
	var marketplace marketplace_query.ItemQuery

	// log.Println(common.MarketplaceType_name[int32(payload.MpType)], "asdasdasd")

	switch payload.MpType {
	case common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE:
		mpquery := marketplace_query.NewMarketplaceQuery(w.db, agent)
		username, err := importer.GetShopUsername()
		if err != nil {
			return mp, err
		}
		marketplace = mpquery.ByUsername(uint(teamID), db_models.MpShopee, username)
		mp, err = marketplace.Get()
		if err != nil {
			if errors.Is(err, marketplace_query.ErrMarketplaceNotFound) {
				return mp, fmt.Errorf("marketplace dengan username %s tidak ada", username)
			}

			return mp, err
		}
	case common.MarketplaceType_MARKETPLACE_TYPE_TIKTOK:
		mpquery := marketplace_query.NewMarketplaceQuery(w.db, agent)
		mp, err = mpquery.
			ByID(uint(teamID), uint(payload.MpId)).
			Get()

		if err != nil {
			return mp, err
		}

	case common.MarketplaceType_MARKETPLACE_TYPE_MENGANTAR:
		mpquery := marketplace_query.NewMarketplaceQuery(w.db, agent)
		mp, err = mpquery.
			ByID(uint(teamID), uint(payload.MpId)).
			Get()

		if err != nil {
			return mp, err
		}
	default:
		return mp, fmt.Errorf("marketplace with type %s not supported", payload.MpType)
	}

	if mp == nil {
		return mp, fmt.Errorf("marketplace with id %d not found and nil", payload.MpId)
	}

	return mp, nil
}

func (w *wdServiceImpl) createImporter(tipe common.MarketplaceType, data []byte) (withdrawal_service_v1.WdImporterIterate, error) {
	var err error
	var importer withdrawal_service_v1.WdImporterIterate

	switch tipe {
	case common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE:
		importer = datasource.NewShopeeWdXls(io.NopCloser(bytes.NewReader(data)))
	case common.MarketplaceType_MARKETPLACE_TYPE_TIKTOK:
		importer = datasource_v2.NewTiktokWdXls(io.NopCloser(bytes.NewReader(data)))
	case common.MarketplaceType_MARKETPLACE_TYPE_MENGANTAR:
		importer = datasource.NewMengantarWdCsv(io.NopCloser(bytes.NewReader(data)))
	default:
		return importer, fmt.Errorf("%s not supported", tipe)
	}

	return importer, err
}
