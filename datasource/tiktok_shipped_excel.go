package datasource

import (
	"archive/zip"
	"bytes"
	"context"
	"io"

	"github.com/pdcgo/shared/pkg/excel_reader"
)

type TiktokShippedItem struct {
	OrderID string `xls:"0"`
	Status  string `xls:"1"`
}

type TiktokShippedXls struct {
	reader io.ReadCloser
}

// Iterate implements order_api.ImporterIterate.
func (t *TiktokShippedXls) Iterate(ctx context.Context, handler func(orderRefID string) error) error {
	buff := bytes.NewBuffer([]byte{})
	size, err := io.Copy(buff, t.reader)
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

		err = sheet.IterWithInterface(ctx, &TiktokShippedItem{}, func(data []string, rowerr error) error {
			if data[0] == "Platform unique order ID." {
				startProcessing = true
				return nil
			}

			if !startProcessing {
				return nil
			}

			item := TiktokShippedItem{}
			err = excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
			if err != nil {
				return err
			}
			if item.Status != "Shipped" {
				return nil
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

func NewTiktokShippedXls(reader io.ReadCloser) *TiktokShippedXls {
	return &TiktokShippedXls{
		reader: reader,
	}
}
