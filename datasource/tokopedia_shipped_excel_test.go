package datasource_test

import (
	"context"
	"os"
	"testing"

	"github.com/pdcgo/withdrawal_service/datasource"
	"github.com/stretchr/testify/assert"
)

func TestDatasourceTokopediaExcel(t *testing.T) {
	fname := "../test/assets/tokopedia_shipping.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTokopediaShippedXls(file)
	c := 0
	importer.Iterate(context.Background(), func(orderRefID string) error {
		c += 1
		return nil
	})

	assert.Equal(t, 3, c)

}
