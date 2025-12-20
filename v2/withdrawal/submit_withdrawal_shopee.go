package withdrawal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"

	"connectrpc.com/connect"
	"github.com/pdcgo/schema/services/order_iface/v1"
	"github.com/pdcgo/schema/services/revenue_iface/v1"
	"github.com/pdcgo/schema/services/withdrawal_iface/v2"
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/withdrawal_service/v2/datasource_shopee"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SubmitWithdrawalShopee implements withdrawal_ifaceconnect.WithdrawalServiceHandler.
func (w *wdServiceImpl) SubmitWithdrawalShopee(
	ctx context.Context,
	req *connect.Request[withdrawal_iface.SubmitWithdrawalShopeeRequest],
	stream *connect.ServerStream[withdrawal_iface.SubmitWithdrawalShopeeResponse],
) error {
	var err error
	identity := w.
		auth.
		AuthIdentityFromHeader(req.Header())

	agent := identity.Identity()

	err = identity.
		Err()

	if err != nil {
		return err
	}

	pay := req.Msg
	// db := w.db.WithContext(ctx)

	streamlog := func(format string, a ...any) error {
		return stream.Send(&withdrawal_iface.SubmitWithdrawalShopeeResponse{
			Message: fmt.Sprintf(format, a...),
		})
	}

	streamerr := func(err error) error {
		if err == nil {
			return nil
		}
		streamlog(err.Error())
		return err
	}

	streamlog("membaca file..")
	var data []byte
	data, err = w.storage.GetContent(ctx, pay.ResourceUri)
	if err != nil {
		return streamerr(fmt.Errorf("error reading %s", pay.ResourceUri))
	}

	// open di datasource baru
	streamlog("parsing file..")
	source := datasource_shopee.NewShopeeXlsWithdrawal(io.NopCloser(bytes.NewReader(data)))

	streamlog("check toko dan marketplace ..")
	var mp *db_models.Marketplace
	mp, err = w.checkShop(
		source,
		uint(pay.TeamId),
		pay.MpSubmit,
		agent,
	)
	if err != nil {
		return streamerr(err)
	}

	streamlog("marketplace %s found..", mp.MpName)

	streamlog("change marketplace id jika tidak sesuai..")
	refids, err := source.GetRefIDs()
	if err != nil {
		streamlog(err.Error())
		return err
	}

	err = w.
		db.
		Model(&db_models.Order{}).
		Where("team_id = ?", pay.TeamId).
		Where("order_ref_id IN ?", refids).
		Where("order_mp_id != ?", mp.ID).
		Update("order_mp_id", mp.ID).
		Error

	if err != nil {
		streamlog(err.Error())
		return err
	}

	streamlog("connecting to revenue service...")
	revenueStream := w.rclient.RevenueStream(ctx)
	revenueStream.Send(&revenue_iface.RevenueStreamRequest{
		Event: &revenue_iface.RevenueStreamEvent{
			Kind: &revenue_iface.RevenueStreamEvent_Init{
				Init: &revenue_iface.RevenueStreamEventInit{
					Token:  req.Header().Get("Authorization"),
					TeamId: pay.TeamId,
					ShopId: pay.MpSubmit.MpId,
					UserId: uint64(agent.IdentityID()),
				},
			},
		},
	})

	wds, err := source.ValidWithdrawal(ctx)
	if err != nil {
		return streamerr(err)
	}

	for _, wd := range wds {
		// withdrawal

		wdAmount := wd.Withdrawal.Amount
		timeStr := wd.Withdrawal.TransactionDate.Format("2006-01-02 15:04:05")
		streamlog("revenue withdrawal amount %.3f at %s", wdAmount, timeStr)
		err = revenueStream.Send(&revenue_iface.RevenueStreamRequest{
			Event: &revenue_iface.RevenueStreamEvent{
				Kind: &revenue_iface.RevenueStreamEvent_Withdrawal{
					Withdrawal: &revenue_iface.RevenueStreamEventWithdrawal{
						Amount: math.Abs(wdAmount),
						At:     timestamppb.New(wd.Withdrawal.TransactionDate),
						Desc:   fmt.Sprintf("tiktok withdrawal withdrawal amount %.3f at %s", wdAmount, timeStr),
					},
				},
			},
		})

		if err != nil {
			return err
		}

		// setting order
		stream := w.orderService.OrderFundSet(ctx)
		for _, earning := range wd.Earning {
			streamlog("add fund to order %s amount %.3f", earning.ExternalOrderID, earning.Amount)
			switch earning.Type {
			case db_models.AdjOrderFund:
				if earning.Amount < 0 {
					continue
				}

				err = stream.Send(&order_iface.OrderFundSetRequest{
					Kind: &order_iface.OrderFundSetRequest_OrderFundSet{
						OrderFundSet: &order_iface.OrderFundSet{
							TeamId: pay.TeamId,
							OrderIdentifier: &order_iface.OrderFundSet_OrderRefId{
								OrderRefId: earning.ExternalOrderID,
							},
							Amount: earning.Amount,
							At:     timestamppb.New(earning.TransactionDate),
							Desc:   earning.Description,
						},
					},
				})
				if err != nil {
					return streamerr(err)
				}

				// complete order
				err = stream.Send(&order_iface.OrderFundSetRequest{
					Kind: &order_iface.OrderFundSetRequest_OrderCompletedSet{
						OrderCompletedSet: &order_iface.OrderCompletedSet{
							TeamId: pay.TeamId,
							OrderIdentifier: &order_iface.OrderCompletedSet_OrderRefId{
								OrderRefId: earning.ExternalOrderID,
							},
							Amount: earning.Amount,
							WdAt:   timestamppb.New(wd.Withdrawal.TransactionDate),
						},
					},
				})
				if err != nil {
					return streamerr(err)
				}
				// case db_models.AdjCommision, db_models.AdjLostCompensation, db_models.AdjCompensation:
				// 	err = processor.OrderAdjustment(item)
				// case db_models.AdjUnknown, db_models.AdjUnknownAdj:
				// 	err = processor.Unknown(item)
			}
		}

		_, err = stream.CloseAndReceive()
		if err != nil {
			return streamerr(err)
		}

	}

	// streaming revenue
	for _, wd := range wds {
		for _, earn := range wd.Earning {
			var ord *db_models.Order
			ord, err = w.orderRepo.OrderByExternalID(earn.ExternalOrderID)
			if err != nil {
				return err
			}

			if ord.ID == 0 {
				return streamerr(fmt.Errorf("cannot get order by order id %s", earn.ExternalOrderID))
			}

			switch earn.Type {
			case db_models.AdjOrderFund:
				switch earn.Amount {
				case -350.00:

				}

				if earn.Amount < 0 {
					continue
				}

				estAmount := float64(ord.OrderMpTotal)
				if estAmount == 0 {
					estAmount = earn.Amount
				}

				// send to revenue
				err = revenueStream.Send(&revenue_iface.RevenueStreamRequest{
					Event: &revenue_iface.RevenueStreamEvent{
						Kind: &revenue_iface.RevenueStreamEvent_Fund{
							Fund: &revenue_iface.RevenueStreamEventFund{
								EstAmount: estAmount,
								Amount:    earn.Amount,
								At:        timestamppb.New(earn.TransactionDate),
								Desc:      fmt.Sprintf("%s on order %s", earn.Description, earn.ExternalOrderID),
								OrderId:   earn.ExternalOrderID,
							},
						},
					},
				})
				if err != nil {
					return streamerr(err)
				}

			case db_models.AdjReturn:
				return streamerr(fmt.Errorf("%s not implemented", earn.Type))
			case db_models.AdjCommision,
				db_models.AdjLostCompensation,
				db_models.AdjCompensation,
				db_models.AdjUnknown,
				db_models.AdjUnknownAdj:

				return streamerr(fmt.Errorf("%s not implemented", earn.Type))

			}
		}

	}

	_, err = revenueStream.CloseAndReceive()
	if err != nil {
		return streamerr(err)
	}

	return nil
}
