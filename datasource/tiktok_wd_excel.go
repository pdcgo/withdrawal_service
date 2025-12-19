package datasource

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/excel_reader"
	"github.com/xuri/excelize/v2"
)

type TiktokWdXls interface {
	Iterate(ctx context.Context, handler func(item *db_models.InvoItem) error) error
	GetRefIDs() (OrderRefList, error)
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

type withdrawalMap interface {
	Add(item *TiktokDayWDItem)
	WdEmitted(item *TiktokWdItem) (bool, *TiktokDayWDItem, error)
	WdNotEmitted() []*TiktokDayWDItem
	calculateAfterAmount() error
}

func NewWithdrawalMap() withdrawalMap {
	return &withdrawalMapImpl{
		data:     []*TiktokDayWDItem{},
		haveEmit: map[string]bool{},
	}
}

type withdrawalMapImpl struct {
	lastwd   *TiktokDayWDItem
	ind      int
	data     []*TiktokDayWDItem
	haveEmit map[string]bool
	wds      []*TiktokDayWDItem
}

func (w *withdrawalMapImpl) calculateAfterAmount() error {
	// mapping sisa
	for _, item := range w.data {
		wd := item.WD
		if wd == nil {
			continue
		}

		wd.DiffAmount += item.Amount
	}
	wlen := len(w.wds)

	for i, wd := range w.wds {
		if (i + 1) == wlen {
			continue
		}
		nextwd := w.wds[i+1]
		nextwd.AfterAmount = (wd.DiffAmount * -1) + wd.AfterAmount
	}

	return nil
}

// Add implements withdrawalMap.
func (w *withdrawalMapImpl) Add(dd *TiktokDayWDItem) {
	item := dd

	switch item.Type {
	case TiktokWDWithdrawal:
		w.lastwd = item
		w.wds = append(w.wds, item)
	case TiktokWDRecordEarning:
		item.WD = w.lastwd
		w.data = append(w.data, item)
	}

}

func (w *withdrawalMapImpl) WdNotEmitted() []*TiktokDayWDItem {
	hasil := []*TiktokDayWDItem{}
	for _, item := range w.wds {
		wd := item
		if !w.haveEmit[wd.SuccessTime.String()] {
			hasil = append(hasil, wd)
		}
	}
	return hasil
}

func (w *withdrawalMapImpl) getStack(pos int, item *TiktokWdItem) (*TiktokDayWDItem, error) {
	if pos < 0 {
		pos = 0
	}

	if (pos + 1) > len(w.data) {
		return nil, fmt.Errorf("stack nil, mungkin xls bermasalah coba download dengan rentan waktu lebih panjang")
	}

	stack := w.data[pos]
	if stack == nil {
		return nil, fmt.Errorf("stack nil, mungkin xls bermasalah")
	}

	if stack.SuccessTime.Equal(item.OrderSettledTime) {
		if item.SettlementAmount != 0 {
			stack.Amount -= item.SettlementAmount
			if stack.Amount == 0 {
				w.ind += 1
			}
		}

	} else {
		// coba getting stack dari previous
		if item.SettlementAmount == 0 {
			stack := w.data[w.ind-1]
			if stack == nil {
				return nil, fmt.Errorf("stack nil, mungkin xls bermasalah2")
			}

			if stack.SuccessTime.Equal(item.OrderSettledTime) {
				return stack, nil
			}
		}
		return nil, fmt.Errorf("%s with %s on stack %s", item.ExternalOrderID, item.OrderSettledTime.String(), stack.SuccessTime.String())
	}

	return stack, nil
}

// WdEmitted implements withdrawalMap.
func (w *withdrawalMapImpl) WdEmitted(item *TiktokWdItem) (bool, *TiktokDayWDItem, error) {
	stack, err := w.getStack(w.ind, item)
	if err != nil {
		return false, nil, err
	}

	if stack.WD != nil {
		if w.haveEmit[stack.WD.SuccessTime.String()] {
			return false, nil, nil
		} else {
			w.haveEmit[stack.WD.SuccessTime.String()] = true
			return true, stack.WD, nil
		}
	}

	return false, nil, nil
}

type tiktokWdXlsImpl struct {
	reader          io.ReadCloser
	f               *excelize.File
	withdrawalMap   withdrawalMap
	orderMetaHeader []string
}

// GetShopUsername implements TiktokWdXls.
func (s *tiktokWdXlsImpl) GetShopUsername() (string, error) {
	return "", ErrCannotGetMarketplaceUsername
}

// GetRefIDs implements TiktokWdXls.
func (s *tiktokWdXlsImpl) GetRefIDs() (OrderRefList, error) {
	var err error
	hasil := OrderRefList{}

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

func NewTiktokWdXls(reader io.ReadCloser) TiktokWdXls {
	return &tiktokWdXlsImpl{
		reader:          reader,
		withdrawalMap:   NewWithdrawalMap(),
		orderMetaHeader: []string{},
	}
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

func (s *tiktokWdXlsImpl) Iterate(ctx context.Context, handler func(item *db_models.InvoItem) error) error {
	// mapping withdrawal dulu
	err := s.mappingWithdrawalAndEarning()
	if err != nil {
		return err
	}

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
		var tipe db_models.AdjustmentType = db_models.AdjUnknown
		switch item.Type {
		case "Order":
			tipe = db_models.AdjOrderFund
			if item.SettlementAmount < 0 {
				tipe = db_models.AdjReturn
			}

			if item.SettlementAmount == 0 {
				return nil
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

		// debugtool.LogJson(invo)

		isEmit, wd, err := s.withdrawalMap.WdEmitted(&item)
		if err != nil {
			return err
		}
		if isEmit {
			err = handler(&db_models.InvoItem{
				MpFrom:          db_models.OrderMpTiktok,
				TransactionDate: wd.SuccessTime,
				Type:            db_models.AdjFund,
				Amount:          wd.Amount,
				BalanceAfter:    wd.AfterAmount,
			})

			if err != nil {
				return err
			}
		}
		return handler(invo)
	})

	if err != nil {
		return err
	}

	wdnotEmit := s.withdrawalMap.WdNotEmitted()
	for _, wd := range wdnotEmit {
		err = handler(&db_models.InvoItem{
			MpFrom:          db_models.OrderMpTiktok,
			TransactionDate: wd.SuccessTime,
			Type:            db_models.AdjFund,
			Amount:          wd.Amount,
			BalanceAfter:    wd.AfterAmount,
		})

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

func (s *tiktokWdXlsImpl) mappingWithdrawalAndEarning() (err error) {
	startProcessing := false
	err = s.iterateSheet("Withdrawal records", func(data []string) error {

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
		case "Earnings", "Withdrawal":
			s.withdrawalMap.Add(&item)
		}

		return nil
	})

	if err != nil {
		return err
	}

	err = s.withdrawalMap.calculateAfterAmount()
	if err != nil {
		return err
	}

	return nil
}
