package datasource

import (
	"context"
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pdcgo/shared/db_models"
)

type MengantarCsv struct {
	reader  io.ReadCloser
	records [][]string
}

// GetRefIDs implements order_api.WdImporterIterate.
func (m *MengantarCsv) GetRefIDs() (OrderRefList, error) {
	var hasil OrderRefList
	err := m.iter(func(item []string) error {
		refID := strings.ReplaceAll(item[2], "Revenue from order ID ", "")
		hasil = append(hasil, refID)
		return nil
	})
	return hasil, err
}

// GetShopUsername implements order_api.WdImporterIterate.
func (m *MengantarCsv) GetShopUsername() (string, error) {
	return "", ErrCannotGetMarketplaceUsername
}

// Iterate implements order_api.WdImporterIterate.
func (m *MengantarCsv) Iterate(ctx context.Context, handler func(item *db_models.InvoItem) error) error {
	layout := "02 Jan 2006 15:04"

	ordfunds := []*db_models.InvoItem{}

	// parsing funds
	err := m.iter(func(item []string) error {
		t, err := time.ParseInLocation(layout, item[1], time.Local)
		if err != nil {
			return err
		}
		refID := strings.ReplaceAll(item[2], "Revenue from order ID ", "")

		amount, err := strconv.ParseFloat(item[14], 64)
		if err != nil {
			return err
		}

		orditem := db_models.InvoItem{
			MpFrom:          db_models.OrderMengantar,
			ExternalOrderID: refID,
			Type:            db_models.AdjOrderFund,
			TransactionDate: t,
			Description:     item[2],
			Amount:          amount,
		}

		ordfunds = append(ordfunds, &orditem)

		return nil
	})

	for _, dd := range ordfunds {
		item := dd

		fund := &db_models.InvoItem{
			MpFrom:          db_models.OrderMengantar,
			Type:            db_models.AdjFund,
			TransactionDate: item.TransactionDate,
			Amount:          item.Amount,
		}

		err = handler(fund)
		if err != nil {
			return err
		}

		err = handler(item)
		if err != nil {
			return err
		}
	}

	return err
}

func (m *MengantarCsv) iter(handler func(item []string) error) error {
	var err error
	if m.records == nil {
		reader := csv.NewReader(m.reader)
		m.records, err = reader.ReadAll()
		if err != nil {
			return nil
		}
	}

	first := true

	for _, item := range m.records {
		if first {
			first = false
			continue
		}

		err = handler(item)
		if err != nil {
			return err
		}
	}

	return nil

}

func NewMengantarWdCsv(reader io.ReadCloser) *MengantarCsv {
	return &MengantarCsv{
		reader: reader,
	}
}
