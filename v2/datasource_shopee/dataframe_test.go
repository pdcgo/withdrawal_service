package datasource_shopee_test

import (
	"math"
	"os"
	"testing"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/debugtool"
	"github.com/pdcgo/withdrawal_service/v2/datasource_shopee"
	"github.com/stretchr/testify/assert"
)

func TestSeries(t *testing.T) {
	s := datasource_shopee.Series[int]{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20}
	check := 0
	for i, item := range s {
		check += item
		t.Log(i+1, check)
		result := s.SearchPosition(func(partial datasource_shopee.Series[int]) (bool, bool) {
			res := 0
			for _, item := range partial {
				res += item
			}
			return res > check, res == check
		})

		offset := []int{}
		for i := range s {
			offset = append(offset, i)
		}

		newOffset := result(offset)

		cek := 0
		for _, i := range newOffset {

			cek += s[i]
		}

		assert.Equal(t, check, cek)
	}

}

func TestDataframe(t *testing.T) {

	fname := "../../test/assets/shopee/mb_erna_err.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource_shopee.NewShopeeXlsWithdrawal(file)
	wds, err := importer.Withdrawals(t.Context())
	assert.Nil(t, err)

	assert.Len(t, wds, 3)

	datas := []*db_models.InvoItem{}
	for _, wd := range wds {
		datas = append(datas, wd.Withdrawal)
		for _, earn := range wd.Earning {
			datas = append(datas, earn)
		}
	}

	df := datasource_shopee.NewInvoListDataframe(datas)

	result := df.Query(
		df.D.Type.Filter(func(i int, item db_models.AdjustmentType) bool {
			return item == db_models.AdjFund
		}),
	)

	stageWds := result.Data()
	wd := stageWds[1]

	fund := df.
		Query(
			df.D.TransactionDate.Filter(func(i int, item time.Time) bool {
				return item.Before(wd.TransactionDate)
			}),
		)

	startFunded := fund.
		Query(
			fund.D.BalanceAfter.Filter(func(i int, item float64) bool {
				return math.Abs(wd.Amount) == item
			}),
		).First()

	funded := fund.
		Query(
			fund.D.TransactionDate.Filter(func(i int, item time.Time) bool {
				return item.Before(startFunded.TransactionDate) || item.Equal(startFunded.TransactionDate)
			}),
			fund.D.BalanceAfter.Break(false, func(i int, item float64) bool {
				return item == 0
			}),
		).
		Data()

	debugtool.LogJson(funded)

	assert.Nil(t, err)
	assert.Len(t, result.Data(), 3)

}
