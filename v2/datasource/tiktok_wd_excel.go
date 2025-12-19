package datasource

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/excel_reader"
	"github.com/pdcgo/withdrawal_service/datasource"
	"github.com/xuri/excelize/v2"
)

type TiktokWdXls interface {
	Iterate(ctx context.Context, handler func(item *db_models.InvoItem) error) error
	GetRefIDs() (datasource.OrderRefList, error)
	GetShopUsername() (string, error)
}

type TiktokWdItem struct {
	ExternalOrderID  string    `xls:"38" xlsheader:"Related order ID"`
	Type             string    `xls:"1"`
	SettlementAmount float64   `xls:"5"`
	OrderSettledTime time.Time `xls:"3" xlsdate:"2006/01/02" addhour:"true"`
}

type TiktokWDRecordType string

const (
	TiktokWDRecordEarning TiktokWDRecordType = "Earnings"
	TiktokWDWithdrawal    TiktokWDRecordType = "Withdrawal"
)

type TiktokDayWDItem struct {
	Type        TiktokWDRecordType `xls:"0"`
	RequestTime time.Time          `xls:"2" xlsdate:"2006/01/02" addhour:"true"`
	Amount      float64            `xls:"3"`
	CAmount     float64            `xls:"3"`
	DiffAmount  float64            `xls:"3"`
	AfterAmount float64
	Status      string    `xls:"4"`
	SuccessTime time.Time `xls:"5" xlsdate:"2006/01/02" addhour:"true"`
	WD          *TiktokDayWDItem
}

type tiktokWdXlsImpl struct {
	reader          io.ReadCloser
	f               *excelize.File
	orderMetaHeader []string
}

// GetRefIDs implements TiktokWdXls.
func (s *tiktokWdXlsImpl) GetRefIDs() (datasource.OrderRefList, error) {
	var err error
	hasil := datasource.OrderRefList{}

	err = s.iterateSheet("Order details", func(data []string) error {
		if data[4] != "IDR" {
			s.getMetaOrderHeader(data)
			return nil
		}

		item := TiktokWdItem{}
		err = excel_reader.UnmarshalRow(&item, data, s.orderMetaHeader)
		if err != nil {
			return err
		}
		hasil.Add(item.ExternalOrderID)
		return nil
	})

	return hasil, err
}

// GetShopUsername implements TiktokWdXls.
func (t *tiktokWdXlsImpl) GetShopUsername() (string, error) {
	return "", ErrCannotGetMarketplaceUsername
}

// Iterate implements TiktokWdXls.
func (t *tiktokWdXlsImpl) Iterate(ctx context.Context, handler func(item *db_models.InvoItem) error) error {
	var err error

	err = t.iterateSheet("Order details", func(data []string) error {

		if data[4] != "IDR" {
			t.getMetaOrderHeader(data)
			return nil
		}

		item := TiktokWdItem{}
		err = excel_reader.UnmarshalRow(&item, data, t.orderMetaHeader)
		if err != nil {
			return err
		}
		var tipe db_models.AdjustmentType = db_models.AdjUnknown
		switch item.Type {
		case "Order":
			tipe = db_models.AdjOrderFund
			if item.SettlementAmount < 0 {
				tipe = db_models.AdjReturn
			}

		case "Logistics reimbursement":
			tipe = db_models.AdjCompensation
		default:
			tipe = db_models.AdjUnknown
		}

		invo := &db_models.InvoItem{
			MpFrom:          db_models.OrderMpTiktok,
			ExternalOrderID: item.ExternalOrderID,
			TransactionDate: item.OrderSettledTime,
			Description:     item.Type,
			Amount:          item.SettlementAmount,
			BalanceAfter:    0,
			Type:            tipe,
		}

		if invo.Amount == 0 {
			return nil
		}

		return handler(invo)
	})

	if err != nil {
		return err
	}

	startProcessing := false
	err = t.iterateSheet("Withdrawal records", func(data []string) error {

		if data[0] == "Type" && data[1] == "Reference ID" {
			startProcessing = true
			return nil
		}

		if !startProcessing {
			return nil
		}

		if data[0] == "" {
			return nil
		}

		switch data[0] {
		case "Withdrawal":
			switch data[4] {
			case "In Process":
				return ErrContainInProcessWD
			case "Failed":
				return nil
			}
		}

		item := TiktokDayWDItem{}
		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		if err != nil {
			return err
		}

		switch item.Type {
		case "Withdrawal":
			invo := &db_models.InvoItem{
				MpFrom:          db_models.OrderMpTiktok,
				TransactionDate: item.SuccessTime,
				Type:            db_models.AdjFund,
				Amount:          item.Amount,
				BalanceAfter:    item.AfterAmount,
				Description:     "penarikan dana tiktok",
			}

			if invo.Amount == 0 {
				return nil
			}

			err = handler(invo)
			if err != nil {
				return err
			}
		}

		return err
	})

	if err != nil {
		return err
	}

	return nil
}

func (s *tiktokWdXlsImpl) iterateSheet(key string, handler func(data []string) error) error {

	f, err := s.getReader()
	if err != nil {
		return err
	}

	rows, err := f.GetRows(key)
	if err != nil {
		return err
	}
	for _, row := range rows {
		err = handler(row)
		if err != nil {
			return err
		}
	}

	return nil
}

func (s *tiktokWdXlsImpl) getReader() (*excelize.File, error) {
	if s.f != nil {
		return s.f, nil
	}

	var err error
	s.f, err = excelize.OpenReader(s.reader)
	return s.f, err
}

func (s *tiktokWdXlsImpl) getMetaOrderHeader(data []string) error {
	// cleaning
	headers := make([]string, len(data))
	for i, d := range data {
		d = strings.Trim(d, " ")
		d = strings.Trim(d, "\t")
		d = strings.Trim(d, "\n")
		headers[i] = d
	}

	if len(headers) == 0 {
		return nil
	}

	if headers[0] != "Order/adjustment ID" {
		return nil
	}

	s.orderMetaHeader = headers

	return nil
}

func NewTiktokWdXls(reader io.ReadCloser) TiktokWdXls {
	return &tiktokWdXlsImpl{
		reader:          reader,
		orderMetaHeader: []string{},
	}
}
