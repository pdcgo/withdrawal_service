package datasource_test

import (
	"context"
	"os"
	"testing"

	"github.com/pdcgo/withdrawal_service/datasource"
	"github.com/stretchr/testify/assert"
)

func TestDatasourceTiktokExcel(t *testing.T) {
	fname := "../test/assets/tiktok_shipping.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokShippedXls(file)
	c := 0
	importer.Iterate(context.Background(), func(orderRefID string) error {
		c += 1
		return nil
	})

	assert.Equal(t, 5, c)

}
