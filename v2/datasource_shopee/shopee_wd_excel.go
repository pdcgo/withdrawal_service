package datasource_shopee

import (
	"context"
	"errors"
	"io"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/debugtool"
	"github.com/pdcgo/shared/pkg/excel_reader"
	"github.com/pdcgo/withdrawal_service/models"
	"github.com/xuri/excelize/v2"
)

var ErrContainInProcessWD = errors.New("ada withdrawal yang masih diproses, silahkan reimport kembali ketika withdrawalnya menjadi selesai")
var ErrCannotGetMarketplaceUsername = errors.New("cannot get marketplace username")

type shopeeXlsImpl struct {
	reader io.ReadCloser
	f      *excelize.File
}

// GetShopUsername implements withdrawal.xlsSource.
func (s *shopeeXlsImpl) GetShopUsername() (string, error) {
	username := ""
	err := s.iterateSheet("Transaction Report", func(data []string) error {
		if data[0] != "Username (Penjual)" {
			return nil
		}

		username = data[1]
		return nil
	})

	return username, err
}

func NewShopeeXlsWithdrawal(reader io.ReadCloser) *shopeeXlsImpl {
	return &shopeeXlsImpl{
		reader: reader,
	}
}

func (s *shopeeXlsImpl) ValidWithdrawal(ctx context.Context) ([]*ShopeeWdSet, error) {
	wds, err := s.Withdrawals(ctx)
	if err != nil {
		return wds, err
	}

	result := []*ShopeeWdSet{}

	for _, wd := range wds {
		notFundedAmount := wd.NotFundedAmount()

		if notFundedAmount != 0 {
			validEarning, err := wd.TraceValidEarning()
			if err != nil {

				if wd.IsLast {
					return result, nil
				}

				return result, err
			}

			result = append(result, &ShopeeWdSet{
				Withdrawal:  wd.Withdrawal,
				WdSetBefore: wd.WdSetBefore,
				WdSetNext:   wd.WdSetNext,
				Earning:     validEarning,
				IsLast:      wd.IsLast,
			})
		}

		if notFundedAmount > 0 {
			debugtool.LogJson(wd)
		}

		if notFundedAmount == 0 {
			result = append(result, wd)
		}
	}

	return result, nil
}

func (s *shopeeXlsImpl) Withdrawals(ctx context.Context) ([]*ShopeeWdSet, error) {
	var err error
	wds := []*ShopeeWdSet{}

	var wd *ShopeeWdSet

	err = s.Iterate(ctx, func(item *db_models.InvoItem) error {
		switch item.Type {
		case db_models.AdjFund:
			oldwd := wd
			wd = &ShopeeWdSet{
				Withdrawal: item,
				Earning:    EarningList{},
				WdSetNext:  oldwd,
			}

			if oldwd != nil {
				oldwd.WdSetBefore = wd
			}

			wds = append(wds, wd)

		default:
			wd.Earning = append(wd.Earning, item)
		}

		return nil
	})

	if err != nil {
		return wds, err
	}

	// setting last
	if wd != nil {
		wd.IsLast = true
	}

	return wds, nil
}

// GetRefIDs implements order_api.WdImporterIterate.
func (s *shopeeXlsImpl) GetRefIDs() (OrderRefList, error) {
	var err error
	hasil := OrderRefList{}

	var startProcessing bool
	s.iterateSheet("Transaction Report", func(data []string) error {
		if data[0] == "Tanggal Transaksi" {
			startProcessing = true
			return nil
		}

		if !startProcessing {
			return nil
		}

		item := models.ShopeeWdItem{}
		err = excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		if err != nil {
			return err
		}
		hasil.Add(item.ExternalOrderID)
		return nil

	})

	return hasil, err

}

// Iterate implements order_api.WdImporterIterate.
func (s *shopeeXlsImpl) Iterate(ctx context.Context, handler func(item *db_models.InvoItem) error) error {
	var err error
	startProcessing := false
	lastitem := models.ShopeeWdItem{}

	return s.iterateSheet("Transaction Report", func(data []string) error {
		if len(data) == 0 {
			return nil
		}
		if data[0] == "Tanggal Transaksi" {
			startProcessing = true
			return nil
		}

		if !startProcessing {
			return nil
		}

		item := models.ShopeeWdItem{}
		err = excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		if err != nil {
			return err
		}
		var tipe db_models.AdjustmentType = db_models.AdjOrderFund
		switch item.Type {
		case models.WdTxFromOrder:
			tipe = db_models.AdjOrderFund
		case models.WdTxFund:
			switch item.Status {
			case "Sedang Diproses":
				return ErrContainInProcessWD
			case "Gagal":
				return nil
			}

			switch item.Description {
			case "Pengembalian Dana untuk Penarikan Gagal":
				return nil
			}

			tipe = db_models.AdjFund
		case models.WdTxAdjustment:
			tipe = db_models.AdjUnknownAdj
			if item.IsCommisionAdjustment() {
				tipe = db_models.AdjCommision
			}
			if item.IsCompensationAdjustment() {
				tipe = db_models.AdjCompensation
			}

			if item.IsLostCompensationAdjustment() {
				tipe = db_models.AdjLostCompensation
			}

			if tipe == db_models.AdjUnknownAdj {
				tipe, _ = item.TryParseOtherType()
			}

			switch item.ExternalOrderID {
			case "", "-":
				refID := item.OrderIDExtractFromDesc()
				if refID != "" {
					item.ExternalOrderID = refID
				}

			}

		default:
			tipe = db_models.AdjUnknown
		}

		// checking duplicate fund
		if item.Type == models.WdTxFund {
			if lastitem.Type == item.Type {
				if lastitem.Amount == item.Amount {
					return nil
				}
			}
		}

		lastitem = item
		return handler(&db_models.InvoItem{
			MpFrom:          db_models.OrderMpShopee,
			ExternalOrderID: item.ExternalOrderID,
			TransactionDate: item.TransactionDate,
			Description:     item.Description,
			Amount:          item.Amount,
			BalanceAfter:    item.BalanceAfter,
			Type:            tipe,
		})

	})
}

func (s *shopeeXlsImpl) getReader() (*excelize.File, error) {
	if s.f != nil {
		return s.f, nil
	}

	var err error
	s.f, err = excelize.OpenReader(s.reader)
	return s.f, err
}

func (s *shopeeXlsImpl) iterateSheet(key string, handler func(data []string) error) error {

	f, err := s.getReader()
	if err != nil {
		return err
	}

	rows, err := f.GetRows(key)
	if err != nil {
		return err
	}
	for _, row := range rows {
		if len(row) == 0 {
			continue
		}
		err = handler(row)
		if err != nil {
			return err
		}
	}

	return nil
}
