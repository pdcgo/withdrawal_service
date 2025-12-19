package datasource_test

import (
	"context"
	"os"
	"testing"

	"github.com/pdcgo/withdrawal_service/datasource"
	"github.com/stretchr/testify/assert"
)

func TestDatasourceShopeeExcel(t *testing.T) {
	fname := "../test/assets/shopee_shipping.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeShippedXls(file)
	c := 0
	importer.Iterate(context.Background(), func(orderRefID string) error {
		c += 1
		return nil
	})

	assert.Equal(t, 193, c)

}
