package datasource_shopee_test

import (
	"os"
	"testing"

	"github.com/pdcgo/withdrawal_service/v2/datasource_shopee"
	"github.com/stretchr/testify/assert"
)

func TestShopeeDatasource(t *testing.T) {
	fname := "../../test/assets/shopee/seluna_selesai_sisa.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource_shopee.NewShopeeXlsWithdrawal(file)
	wds, err := importer.ValidWithdrawal(t.Context())
	assert.Nil(t, err)

	assert.Len(t, wds, 20)

	// for _, wd := range wds {
	// 	t.Logf("%.3f", wd.Withdrawal.Amount)
	// }

	// for _, wd := range wds {
	// 	assert.Equal(t, math.Abs(wd.Withdrawal.Amount), wd.Earning.GetAmount())
	// 	debugtool.LogJson(wd)
	// }

}

// 19/12/2025
func TestShopeeReturnAwan(t *testing.T) {
	fname := "../../test/assets/shopee/awan_beban_return.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource_shopee.NewShopeeXlsWithdrawal(file)
	wds, err := importer.ValidWithdrawal(t.Context())
	assert.Nil(t, err)

	assert.Len(t, wds, 15)

	// for _, wd := range wds {
	// 	for _, earn := range wd.Earning {
	// 		switch earn.Amount {
	// 		case -350.00:
	// 			debugtool.LogJson(earn)
	// 		}
	// 	}
	// }

}

func TestPanicVioleta(t *testing.T) {
	fname := "../../test/assets/shopee/panic_violeta.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource_shopee.NewShopeeXlsWithdrawal(file)
	_, err = importer.ValidWithdrawal(t.Context())
	assert.Nil(t, err)
}
