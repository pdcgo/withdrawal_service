package datasource

import (
	"context"
	"io"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/excel_reader"
	"github.com/pdcgo/withdrawal_service/models"
	"github.com/xuri/excelize/v2"
)

type ShopeeWdXls struct {
	f      *excelize.File
	reader io.ReadCloser
}

// GetShopUsername implements order_api.WdImporterIterate.
func (s *ShopeeWdXls) GetShopUsername() (string, error) {
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

// GetRefIDs implements order_api.WdImporterIterate.
func (s *ShopeeWdXls) GetRefIDs() (OrderRefList, error) {
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
func (s *ShopeeWdXls) Iterate(ctx context.Context, handler func(item *db_models.InvoItem) error) error {
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

func (s *ShopeeWdXls) getReader() (*excelize.File, error) {
	if s.f != nil {
		return s.f, nil
	}

	var err error
	s.f, err = excelize.OpenReader(s.reader)
	return s.f, err
}

func (s *ShopeeWdXls) iterateSheet(key string, handler func(data []string) error) error {

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

func NewShopeeWdXls(reader io.ReadCloser) *ShopeeWdXls {
	return &ShopeeWdXls{
		reader: reader,
	}
}
