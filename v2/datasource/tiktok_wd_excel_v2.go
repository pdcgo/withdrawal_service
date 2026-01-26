package datasource

import (
	"fmt"
	"io"
	"strings"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/excel_reader"
	"github.com/xuri/excelize/v2"
)

type v2TiktokWdImpl struct {
	reader          io.ReadCloser
	f               *excelize.File
	orderMetaHeader []string
}

func NewV2TiktokWdXls(reader io.ReadCloser) *v2TiktokWdImpl {
	return &v2TiktokWdImpl{
		reader: reader,
	}
}

func (s *v2TiktokWdImpl) IterateValidWithdrawal() ([]*WdSet, error) {
	var err error
	var wds []*WdSet

	wds, err = s.IterateWithdrawal()
	if err != nil {
		return wds, err
	}
	result := []*WdSet{}

	for _, wd := range wds {

		fundedEarning, _, err := wd.FundedEarning()
		if err != nil {
			if wd.IsLast {
				return result, nil
			}

			return result, err
		}

		if len(fundedEarning) == 0 {
			return result, wd.WithErrf("funded entry empty")
		}

		result = append(result, &WdSet{
			Withdrawal:  wd.Withdrawal,
			WdSetBefore: wd.WdSetBefore,
			WdSetNext:   wd.WdSetNext,
			Earning:     fundedEarning,
			IsLast:      wd.IsLast,
		})
	}

	return result, nil
}

func (s *v2TiktokWdImpl) IterateWithdrawal() ([]*WdSet, error) {
	var err error

	details, err := s.DetailSet()
	if err != nil {
		return nil, err
	}

	wds := []*WdSet{}

	// orderMapDate, err := s.GroupByDate()
	// if err != nil {
	// 	return wds, err
	// }

	startProcessing := false

	var wd *WdSet

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

		var invos InvoItemList

		switch item.Type {
		case "Earnings":
			invos, err = details.GetOrderEarning(item.RequestTime)
			if err != nil {
				return err
			}

			if wd != nil {
				wd.Earning = append(wd.Earning, &Earning{
					Earning:  &item,
					Involist: invos,
				})
			}

		case "GMV Pay Deduction":
			invos, err = details.GetGmvDeduction(item.RequestTime, item.Amount)

			if item.Amount != invos.GetAmount() {
				return fmt.Errorf(
					"error transaction %s amount %.3f time %s",
					item.Type,
					item.Amount,
					item.RequestTime.Format("2006-01-02"),
				)
			}

			if wd != nil {
				wd.Earning = append(wd.Earning, &Earning{
					Earning:  &item,
					Involist: invos,
				})
			}

		case "Withdrawal":
			oldwd := wd
			wd = &WdSet{
				Withdrawal: &item,
				Earning:    []*Earning{},
				WdSetNext:  oldwd,
			}

			if oldwd != nil {
				oldwd.WdSetBefore = wd
			}

			wds = append(wds, wd)
		}

		return nil
	})

	// setting last
	if wd != nil {
		wd.IsLast = true
	}

	return wds, err
}

type InvoItemList []*db_models.InvoItem

func (invs InvoItemList) AdsPayment() InvoItemList {
	hasil := InvoItemList{}
	for _, inv := range invs {
		if inv.Type == db_models.AdsPayment {
			hasil = append(hasil, inv)
		}
	}
	return hasil
}

// func (invs InvoItemList) OrderPayment() InvoItemList {
// 	hasil := InvoItemList{}
// 	for _, inv := range invs {
// 		if inv.Type != db_models.AdsPayment {
// 			hasil = append(hasil, inv)
// 		}
// 	}
// 	return hasil
// }

func (invs InvoItemList) GetAmount() float64 {
	hasil := 0.00
	for _, inv := range invs {
		hasil += inv.Amount
	}
	return hasil
}

func (invs InvoItemList) GetOrderIDs() []string {
	hasil := []string{}
	for _, inv := range invs {
		hasil = append(hasil, inv.ExternalOrderID)
	}
	return hasil
}

func (s *v2TiktokWdImpl) DetailSet() (*DetailSet, error) {
	var err error
	result := DetailSet{[]*db_models.InvoItem{}, map[string]bool{}}

	err = s.IterateOrder(func(invo *db_models.InvoItem) error {
		result.Data = append(result.Data, invo)
		return nil
	})

	return &result, err
}

// func (s *v2TiktokWdImpl) GroupByDate() (map[int64]InvoItemList, error) {
// 	var err error
// 	hasil := map[int64]InvoItemList{}

// 	err = s.IterateOrder(func(invo *db_models.InvoItem) error {
// 		log.Println(invo.Type)
// 		hasil[invo.TransactionDate.Unix()] = append(hasil[invo.TransactionDate.Unix()], invo)
// 		return nil
// 	})

// 	if err != nil {
// 		return hasil, err
// 	}

// 	return hasil, err

// }

func (s *v2TiktokWdImpl) IterateOrder(handler func(invo *db_models.InvoItem) error) error {
	return s.iterateSheet("Order details", func(data []string) error {
		var err error
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

		case "GMV Payment for TikTok Ads":
			tipe = db_models.AdsPayment
			item.ExternalOrderID = data[0]

		case "Logistics reimbursement":
			if item.SettlementAmount > 0 {
				tipe = db_models.AdjOrderFund
			} else {
				tipe = db_models.AdjUnknown
			}

		case "Platform reimbursement":
			if item.SettlementAmount > 0 {
				tipe = db_models.AdjOrderFund
			} else {
				tipe = db_models.AdjUnknown
			}
		default:
			if strings.HasPrefix(item.Type, "wderror") {
				tipe = db_models.InternalWdError
			} else {
				tipe = db_models.AdjUnknown
			}

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
		return handler(invo)
	})

}

func (s *v2TiktokWdImpl) getMetaOrderHeader(data []string) error {
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

func (s *v2TiktokWdImpl) iterateSheet(key string, handler func(data []string) error) error {

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

func (s *v2TiktokWdImpl) getReader() (*excelize.File, error) {
	if s.f != nil {
		return s.f, nil
	}

	var err error
	s.f, err = excelize.OpenReader(s.reader)
	return s.f, err
}
