package withdrawal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"

	"connectrpc.com/connect"
	"github.com/pdcgo/schema/services/accounting_iface/v1"
	"github.com/pdcgo/schema/services/common/v1"
	"github.com/pdcgo/schema/services/order_iface/v1"
	"github.com/pdcgo/schema/services/revenue_iface/v1"
	"github.com/pdcgo/schema/services/withdrawal_iface/v2"
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/withdrawal_service/v2/datasource"
	"google.golang.org/protobuf/types/known/timestamppb"
)

var tiktokDateFmt = "2006-01-02"

// SubmitWithdrawalTiktok implements withdrawal_ifaceconnect.WithdrawalServiceHandler.
func (w *wdServiceImpl) SubmitWithdrawalTiktok(
	ctx context.Context,
	req *connect.Request[withdrawal_iface.SubmitWithdrawalTiktokRequest],
	stream *connect.ServerStream[withdrawal_iface.SubmitWithdrawalTiktokResponse]) error {
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
		return stream.Send(&withdrawal_iface.SubmitWithdrawalTiktokResponse{
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

	streamerrf := func(format string, a ...any) error {
		return streamerr(fmt.Errorf(format, a...))
	}

	streamlog("membaca file..")
	var data []byte
	data, err = w.storage.GetContent(ctx, pay.ResourceUri)
	if err != nil {
		streamlog("error reading %s", pay.ResourceUri)
		return err
	}

	streamlog("check toko dan marketplace ..")
	var mp *db_models.Marketplace
	mp, err = w.checkShop(
		nil,
		uint(pay.TeamId),
		pay.MpSubmit,
		agent,
	)
	if err != nil {
		streamlog(err.Error())
		return err
	}

	streamlog("marketplace %s found..", mp.MpName)

	// open di datasource baru
	streamlog("parsing file..")
	source := datasource.NewV2TiktokWdXls(io.NopCloser(bytes.NewReader(data)))
	wds, err := source.IterateValidWithdrawal()
	if err != nil {
		return streamerr(err)
	}

	// update order jadi selesai
	for _, wd := range wds {
		wdAmount := wd.Withdrawal.Amount
		timeStr := wd.Withdrawal.RequestTime.Format(tiktokDateFmt)
		streamlog("revenue withdrawal amount %.3f at %s", wdAmount, timeStr)
		_, err = w.rclient.Withdrawal(ctx, &connect.Request[revenue_iface.WithdrawalRequest]{
			Msg: &revenue_iface.WithdrawalRequest{
				TeamId: pay.TeamId,
				ShopId: pay.MpSubmit.MpId,
				At:     timestamppb.New(wd.Withdrawal.SuccessTime),
				Amount: math.Abs(wdAmount),
				Desc:   fmt.Sprintf("tiktok withdrawal amount %.3f at %s", wdAmount, timeStr),
			},
		})

		if err != nil {
			return streamerr(err)
		}

		// for _, earning := range wd.Earning {
		// 	for _, inv := range earning.Involist {
		// 		streamlog("add fund to order %s amount %.3f", inv.ExternalOrderID, inv.Amount)
		// 		switch inv.Type {
		// 		case db_models.AdjOrderFund:
		// 			err = stream.Send(&order_iface.OrderFundSetRequest{
		// 				Kind: &order_iface.OrderFundSetRequest_OrderFundSet{
		// 					OrderFundSet: &order_iface.OrderFundSet{
		// 						TeamId: pay.TeamId,
		// 						OrderIdentifier: &order_iface.OrderFundSet_OrderRefId{
		// 							OrderRefId: inv.ExternalOrderID,
		// 						},
		// 						Amount: inv.Amount,
		// 						At:     timestamppb.New(inv.TransactionDate),
		// 						Desc:   inv.Description,
		// 					},
		// 				},
		// 			})
		// 			if err != nil {
		// 				return streamerr(err)
		// 			}

		// 			// complete order
		// 			err = stream.Send(&order_iface.OrderFundSetRequest{
		// 				Kind: &order_iface.OrderFundSetRequest_OrderCompletedSet{
		// 					OrderCompletedSet: &order_iface.OrderCompletedSet{
		// 						TeamId: pay.TeamId,
		// 						OrderIdentifier: &order_iface.OrderCompletedSet_OrderRefId{
		// 							OrderRefId: inv.ExternalOrderID,
		// 						},
		// 						Amount: inv.Amount,
		// 						WdAt:   timestamppb.New(wd.Withdrawal.RequestTime),
		// 					},
		// 				},
		// 			})
		// 			if err != nil {
		// 				return streamerr(err)
		// 			}

		// 		}

		// 	}
		// }

		// streaming to revenue
		for _, earning := range wd.Earning {
			for _, inv := range earning.Involist {

				// getting order
				var ord *db_models.Order
				switch inv.Type {
				case db_models.AdsPayment:
				case db_models.AdjUnknown:
				default:
					ord, err = w.orderRepo.OrderByExternalID(inv.ExternalOrderID)
					if err != nil {
						return err
					}

					if ord.ID == 0 {
						return streamerrf("cannot get order by order id %s", inv.ExternalOrderID)
					}
				}

				req := &order_iface.MpPaymentCreateRequest{
					TeamId:  uint64(ord.TeamID),
					OrderId: uint64(ord.ID),
					ShopId:  uint64(mp.ID),
					Type:    string(inv.Type),
					Amount:  inv.Amount,
					Desc:    inv.Description,
					At:      timestamppb.New(inv.TransactionDate),
					WdAt:    timestamppb.New(wd.Withdrawal.SuccessTime),
					Source:  order_iface.MpPaymentSource_MP_PAYMENT_SOURCE_IMPORTER,
				}

				var paymentCreateRes *connect.Response[order_iface.MpPaymentCreateResponse]

				switch inv.Type {
				case db_models.AdjOrderFund:
					if inv.Amount < 0 {
						return streamerrf("amount fund negative %s", inv.ExternalOrderID)

					}

					streamlog("add fund %s to order %s amount %.3f", inv.Type, inv.ExternalOrderID, inv.Amount)
					paymentCreateRes, err = w.orderService.MpPaymentCreate(ctx, &connect.Request[order_iface.MpPaymentCreateRequest]{
						Msg: req,
					})

					if err != nil {
						return streamerr(err)
					}

					estAmount := float64(ord.OrderMpTotal)
					if estAmount == 0 {
						estAmount = inv.Amount
					}

				case db_models.AdjReturn:
					streamlog("add adjustment %s %.3f %s", inv.Type, inv.Amount, inv.ExternalOrderID)
					paymentCreateRes, err = w.orderService.MpPaymentCreate(ctx, &connect.Request[order_iface.MpPaymentCreateRequest]{
						Msg: req,
					})

					if err != nil {
						return streamerr(err)
					}
				// err = revenueStream.Send(&revenue_iface.RevenueStreamRequest{
				// 	Event: &revenue_iface.RevenueStreamEvent{},
				// })

				// if err != nil {
				// 	return streamerr(err)
				// }
				case db_models.AdsPayment:
					if inv.Amount > 0 {
						continue
					}
					// kasus gmv payment
					_, err = w.adsService.AdsExCreate(ctx, &connect.Request[accounting_iface.AdsExCreateRequest]{
						Msg: &accounting_iface.AdsExCreateRequest{
							TeamId:        pay.TeamId,
							ShopId:        uint64(mp.ID),
							ExternalRefId: inv.TransactionDate.Format(tiktokDateFmt),
							Source:        accounting_iface.AccountSource_ACCOUNT_SOURCE_SHOP,
							MpType:        common.MarketplaceType_MARKETPLACE_TYPE_TIKTOK,
							Amount:        math.Abs(inv.Amount),
							Desc:          inv.Description,
						},
					})

					if err != nil {
						return streamerr(err)
					}

				case db_models.AdjUnknown:
					streamlog("add adjustment %s %s", inv.Type, inv.Description)
					if inv.Amount < 0 {
						_, err := w.rclient.SellingExpenseOther(ctx, &connect.Request[revenue_iface.SellingExpenseOtherRequest]{
							Msg: &revenue_iface.SellingExpenseOtherRequest{
								TeamId:            pay.TeamId,
								ExternalExpenseId: fmt.Sprintf("%s%s", inv.Description, inv.TransactionDate.Format(tiktokDateFmt)),
								LabelInfo: &revenue_iface.ExtraLabelInfo{
									CsId:   uint64(agent.IdentityID()),
									ShopId: uint64(mp.ID),
									TypeLabels: []*accounting_iface.TypeLabel{
										{
											Key:   accounting_iface.LabelKey_LABEL_KEY_REVENUE_SOURCE,
											Label: accounting_iface.RevenueSource_name[int32(accounting_iface.RevenueSource_REVENUE_SOURCE_OTHER)],
										},
									},
								},
								Amount: math.Abs(inv.Amount),
								Desc:   inv.Description,
								At:     timestamppb.New(inv.TransactionDate),
							},
						})

						if err != nil {
							return streamerr(err)
						}
					} else {

						paymentCreateRes, err = w.orderService.MpPaymentCreate(ctx, &connect.Request[order_iface.MpPaymentCreateRequest]{
							Msg: req,
						})

						if err != nil {
							return streamerr(err)
						}

						// _, err := w.rclient.RevenueOther(ctx, &connect.Request[revenue_iface.RevenueOtherRequest]{
						// 	Msg: &revenue_iface.RevenueOtherRequest{
						// 		TeamId:            pay.TeamId,
						// 		ExternalRevenueId: inv.ExternalOrderID,
						// 		LabelInfo: &revenue_iface.ExtraLabelInfo{
						// 			CsId:   uint64(agent.IdentityID()),
						// 			ShopId: uint64(mp.ID),
						// 			TypeLabels: []*accounting_iface.TypeLabel{
						// 				{
						// 					Key:   accounting_iface.LabelKey_LABEL_KEY_REVENUE_SOURCE,
						// 					Label: accounting_iface.RevenueSource_name[int32(accounting_iface.RevenueSource_REVENUE_SOURCE_OTHER)],
						// 				},
						// 			},
						// 		},
						// 		Amount: inv.Amount,
						// 		Desc:   inv.Description,
						// 		At:     timestamppb.New(inv.TransactionDate),
						// 	},
						// })

						// if err != nil {
						// 	return streamerr(err)
						// }
					}

				default:
					return streamerrf("%s not implemented", inv.Type)
				}

				if paymentCreateRes.Msg.IsReceivableCreatedAdjustment {
					streamlog("set finish order %s %t", inv.ExternalOrderID, paymentCreateRes.Msg.IsReceivableCreatedAdjustment)
					_, err = w.orderService.OrderCompleted(ctx, &connect.Request[order_iface.OrderCompletedRequest]{
						Msg: &order_iface.OrderCompletedRequest{
							TeamId:  pay.TeamId,
							OrderId: uint64(ord.ID),
						},
					})

					if err != nil {
						return streamerr(err)
					}
				}
			}
		}

	}

	return nil
}
