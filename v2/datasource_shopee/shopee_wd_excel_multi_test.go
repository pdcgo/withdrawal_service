package datasource_shopee_test

import (
	"io"
	"os"
	"testing"

	"github.com/pdcgo/withdrawal_service/v2/datasource_shopee"
	"github.com/stretchr/testify/assert"
)

func TestOrderLuarNegeri(t *testing.T) {
	fnames := []string{
		"../../test/assets/shopee/shopee_malaysia_base.xlsx",
		"../../test/assets/shopee/shopee_malaysia.xlsx",
	}
	files := []io.ReadCloser{}

	for _, fname := range fnames {
		file, err := os.Open(fname)
		assert.Nil(t, err)
		defer file.Close()

		files = append(files, file)
	}

	importer, err := datasource_shopee.NewShopeeXlsMultiFile(files)
	assert.Nil(t, err)

	_, err = importer.ValidWithdrawal(t.Context())
	assert.Nil(t, err)
}
