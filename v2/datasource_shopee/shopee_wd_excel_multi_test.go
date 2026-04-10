package datasource_shopee_test

import (
	"io"
	"os"
	"testing"

	"github.com/pdcgo/shared/pkg/debugtool"
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

func TestOrderMbakErna(t *testing.T) {
	fnames := []string{
		"../../test/assets/shopee/mb_erna_err.xlsx",
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

	wds, err := importer.ValidWithdrawal(t.Context())
	assert.Nil(t, err)
	assert.Len(t, wds, 2)

	for _, wd := range wds {
		t.Logf("%.3f", wd.Withdrawal.Amount)
	}

}

func TestAwanWdGagal(t *testing.T) {
	fnames := []string{
		"../../test/assets/shopee/awan_wdgagal.xlsx",
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

	wds, err := importer.ValidWithdrawal(t.Context())
	assert.Nil(t, err)
	assert.Len(t, wds, 16)

	for _, wd := range wds {
		t.Logf("%.3f", wd.Withdrawal.Amount)
	}
}

func TestLuxyWdGagal(t *testing.T) {
	fnames := []string{
		"../../test/assets/shopee/luxy_wdgagal.xlsx",
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

	wds, err := importer.ValidWithdrawal(t.Context())
	assert.Nil(t, err)
	assert.Len(t, wds, 4)

	for _, wd := range wds {
		switch wd.Withdrawal.Amount {
		case -3900538:
			debugtool.LogJson(wd.Earning[0])
			debugtool.LogJson(wd.Earning[len(wd.Earning)-1])
		case -3977187:
			t.Error("asdasdasd")
		}
	}
}
