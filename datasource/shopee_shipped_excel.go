package datasource

import (
	"archive/zip"
	"bytes"
	"context"
	"io"

	"github.com/pdcgo/shared/pkg/excel_reader"
)

type ShopeeShippedItem struct {
	OrderID string `xls:"0"`
	Status  string `xls:"1"`
}

type ShopeeShippedXls struct {
	reader io.ReadCloser
}

func NewShopeeShippedXls(reader io.ReadCloser) *ShopeeShippedXls {
	return &ShopeeShippedXls{
		reader: reader,
	}
}

// Iterate implements order_api.ImporterIterate.
func (s *ShopeeShippedXls) Iterate(ctx context.Context, handler func(orderRefID string) error) error {
	buff := bytes.NewBuffer([]byte{})
	size, err := io.Copy(buff, s.reader)
	if err != nil {
		return err
	}

	breader := bytes.NewReader(buff.Bytes())

	archive, err := zip.NewReader(breader, size)
	if err != nil {
		return err
	}

	reader := excel_reader.NewExcelReader(archive)
	workbook, err := reader.GetWorkbook()
	if err != nil {
		return err
	}

	for name := range workbook.Sheets {
		sheet, err := workbook.GetSheet(name)
		if err != nil {
			return err
		}

		startProcessing := false

		err = sheet.IterWithInterface(ctx, &ShopeeShippedItem{}, func(data []string, rowerr error) error {
			if data[0] == "No. Pesanan" {
				startProcessing = true
				return nil
			}

			if !startProcessing {
				return nil
			}

			item := ShopeeShippedItem{}
			err = excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
			if err != nil {
				return err
			}

			handler(item.OrderID)

			return nil
		})

		if err != nil {
			return err
		}
	}

	return nil
}
