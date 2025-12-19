package datasource_test

import (
	"math"
	"os"
	"testing"

	"github.com/pdcgo/withdrawal_service/v2/datasource"
	"github.com/stretchr/testify/assert"
)

func TestCombination(t *testing.T) {

	datasource.ComboIndices(5, func(datas []int) error {
		return nil
	})

}

func TestComboFileExcel(t *testing.T) {
	fname := "../../test/assets/pzen_complete_salah.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewV2TiktokWdXls(file)
	_, err = importer.IterateWithdrawal()
	assert.Nil(t, err)

	wds, err := importer.IterateWithdrawal()
	assert.Nil(t, err)

	notfunded := 0

	for _, wd := range wds {
		if wd.NotFundedAmount() != 0 {
			notfunded += 1

			// earning, _, err := wd.FundedEarning()
			// assert.Nil(t, err)

			// debugtool.LogJson(earning)
		}
	}
	assert.Equal(t, 2, notfunded)

}

func TestToktokSini1Bulan(t *testing.T) {
	fname := "../../test/assets/overlapping_earning.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewV2TiktokWdXls(file)
	_, err = importer.IterateWithdrawal()
	assert.Nil(t, err)

	wds, err := importer.IterateWithdrawal()
	assert.Nil(t, err)

	notfunded := 0

	for _, wd := range wds {

		switch wd.Withdrawal.Amount {
		case -2559626:
			assert.Len(t, wd.Earning, 3)
		case -3958796:
			assert.Len(t, wd.Earning, 2)

			earning, err := wd.TraceValidEarning()
			assert.Nil(t, err)
			assert.Equal(t, math.Abs(wd.Withdrawal.Amount), earning.GetAmount())

		}

		if wd.NotFundedAmount() != 0 {
			notfunded += 1

		}
	}
	assert.Equal(t, 3, notfunded)
}
