package datasource

import (
	"archive/zip"
	"bytes"
	"context"
	"io"

	"github.com/pdcgo/shared/pkg/excel_reader"
)

type TokopediaShippedItem struct {
	OrderID string `xls:"1"`
	Status  string `xls:"3"`
}

type TokopediaShippedXls struct {
	reader io.ReadCloser
}

// Iterate implements order_api.ImporterIterate.
func (l *TokopediaShippedXls) Iterate(ctx context.Context, handler func(orderRefID string) error) error {
	buff := bytes.NewBuffer([]byte{})
	size, err := io.Copy(buff, l.reader)
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
	haveProcess := map[string]bool{}
	for name := range workbook.Sheets {
		sheet, err := workbook.GetSheet(name)
		if err != nil {
			return err
		}

		startProcessing := false

		err = sheet.IterWithInterface(ctx, &TokopediaShippedItem{}, func(data []string, rowerr error) error {
			if data[1] == "Nomor Invoice" {
				startProcessing = true
				return nil
			}

			if !startProcessing {
				return nil
			}

			item := TokopediaShippedItem{}
			err = excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
			if err != nil {
				return err
			}

			if item.Status != "Sedang Dikirim" {
				return nil
			}
			if haveProcess[item.OrderID] {
				return nil
			} else {
				haveProcess[item.OrderID] = true
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

func NewTokopediaShippedXls(reader io.ReadCloser) *TokopediaShippedXls {
	return &TokopediaShippedXls{
		reader: reader,
	}
}
