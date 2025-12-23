package withdrawal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math"
	"time"

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

	wds, err := source.ValidWithdrawal(ctx)
	if err != nil {
		return streamerr(err)
	}

	for _, wd := range wds {
		// withdrawal

		wdAmount := wd.Withdrawal.Amount
		timeStr := wd.Withdrawal.TransactionDate.Format("2006-01-02 15:04:05")
		streamlog("revenue withdrawal amount %.3f at %s", wdAmount, timeStr)
		_, err = w.rclient.Withdrawal(ctx, &connect.Request[revenue_iface.WithdrawalRequest]{
			Msg: &revenue_iface.WithdrawalRequest{
				TeamId: pay.TeamId,
				ShopId: pay.MpSubmit.MpId,
				At:     timestamppb.New(wd.Withdrawal.TransactionDate),
				Amount: math.Abs(wdAmount),
				Desc:   fmt.Sprintf("tiktok withdrawal withdrawal amount %.3f at %s", wdAmount, timeStr),
			},
		})

		if err != nil {
			return streamerr(err)
		}

		// masih ruwet sampai sini
		for _, earn := range wd.Earning {

			var ord *db_models.Order
			ord, err = w.orderRepo.OrderByExternalID(earn.ExternalOrderID)
			if err != nil {
				return err
			}

			if ord.ID == 0 {
				return streamerr(fmt.Errorf("cannot get order by order id %s", earn.ExternalOrderID))
			}

			req := &order_iface.MpPaymentCreateRequest{
				TeamId:  uint64(ord.TeamID),
				OrderId: uint64(ord.ID),
				ShopId:  uint64(mp.ID),
				Type:    string(earn.Type),
				Amount:  earn.Amount,
				Desc:    earn.Description,
				At:      timestamppb.New(earn.TransactionDate),
				WdAt:    timestamppb.New(wd.Withdrawal.TransactionDate),
				Source:  order_iface.MpPaymentSource_MP_PAYMENT_SOURCE_IMPORTER,
			}

			var paymentCreateRes *connect.Response[order_iface.MpPaymentCreateResponse]

			switch earn.Type {
			case db_models.AdjOrderFund:
				// if earn.Amount < 0 {
				// 	return streamerr(wd.WithErr(errors.New("amount fund negative " + earn.ExternalOrderID)))
				// }

				switch earn.Amount {
				case -350.00:
					req.Type = string(db_models.AdjReturn)
				}

				streamlog("add fund %s to order %s amount %.3f", earn.Type, earn.ExternalOrderID, earn.Amount)
				paymentCreateRes, err = w.orderService.MpPaymentCreate(ctx, &connect.Request[order_iface.MpPaymentCreateRequest]{
					Msg: req,
				})

				if err != nil {
					return streamerr(err)
				}

			case db_models.AdjLostCompensation,
				db_models.AdjReturn,
				db_models.AdjCommision,
				db_models.AdjCompensation,
				db_models.AdjUnknown,
				db_models.AdjPremi,
				db_models.AdjUnknownAdj:
				streamlog("add adjustment %s %s", earn.Type, earn.Description)
				paymentCreateRes, err = w.orderService.MpPaymentCreate(ctx, &connect.Request[order_iface.MpPaymentCreateRequest]{
					Msg: req,
				})

				if err != nil {
					return streamerr(err)
				}

			default:
				return streamerr(fmt.Errorf("[withdrawal] %s not implemented", earn.Type))

			}

			if paymentCreateRes.Msg.IsReceivableCreatedAdjustment {
				streamlog("set finish order %s %t", earn.ExternalOrderID, paymentCreateRes.Msg.IsReceivableCreatedAdjustment)
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

	return nil
}

func (w *wdServiceImpl) getOrderAdjustmentMultiRegion(orderID uint, before, after time.Time) ([]*db_models.OrderAdjustment, error) {
	// var err error
	// var adjs []*db_models.OrderAdjustment
	adjs := []*db_models.OrderAdjustment{}
	w.db.
		Model(&db_models.OrderAdjustment{}).
		Where("order_id = ?", orderID).
		Where("is_multi_region = ?", true).
		Where("at BETWEEN ? AND ?", before, after).
		Find(&adjs)

	return adjs, nil
}
