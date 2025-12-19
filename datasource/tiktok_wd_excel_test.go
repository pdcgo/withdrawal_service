package datasource_test

import (
	"context"
	"os"
	"slices"
	"testing"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/debugtool"
	"github.com/pdcgo/shared/pkg/excel_reader"
	"github.com/pdcgo/withdrawal_service/datasource"
	datasource_v2 "github.com/pdcgo/withdrawal_service/v2/datasource"
	"github.com/stretchr/testify/assert"
)

func TestModel(t *testing.T) {
	t.Run("testing tanggal harus bukan utc", func(t *testing.T) {
		data := []string{
			"Earnings",
			"3460994681524225113",
			"2024/09/29",
			"1259997",
			"Transferred",
			"2024/09/29",
			"/",
		}

		item := datasource.TiktokDayWDItem{}
		err := excel_reader.UnmarshalRow(&item, data, excel_reader.MetaIndex{})
		assert.Nil(t, err)

	})

}
func TestTiktokOrderKosong(t *testing.T) {
	fname := "../test/assets/testwd/tt_include_0.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)

	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		assert.NotEqual(t, 0.00, item.Amount)

		return nil
	})

	assert.Nil(t, err)

}

func TestTiktokFianError(t *testing.T) {
	fname := "../test/assets/tiktok_fian_err.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource_v2.NewTiktokWdXls(file)

	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		debugtool.LogJson(item)
		assert.NotEqual(t, item.Amount, 0.00)
		return nil
	})

	assert.Nil(t, err)

}

func TestTiktokGmvPayment(t *testing.T) {
	t.Skip("belum bisa resolve")
	fname := "../test/assets/testwd/tiktok_wd_include_gmv.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)

	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		switch item.Type {
		case db_models.AdjOrderFund:
			assert.Greater(t, item.Amount, 0.00)
		}
		return nil
	})

	assert.Nil(t, err)

}

func TestTiktokNegatif(t *testing.T) {
	fname := "../test/assets/testwd/tiktok_negatif.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)

	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		switch item.Type {
		case db_models.AdjOrderFund:
			assert.Greater(t, item.Amount, 0.00)
		}
		return nil
	})

	assert.Nil(t, err)

}

func TestTiktokReimbushment(t *testing.T) {
	// fname := "../../test/assets/tiktokincome.xlsx"
	fname := "../test/assets/testwd/tiktok_reimbushment.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)

	found := false

	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		if item.Type == db_models.AdjCompensation {
			found = true
			assert.Equal(t, "Logistics reimbursement", item.Description)
		}
		return nil
	})

	assert.Nil(t, err)

	assert.True(t, found)
}

func TestTiktokWdSalsaPanic(t *testing.T) {

	// fname := "../../test/assets/tiktokincome.xlsx"
	fname := "../test/assets/testwd/salsa_tiktok_error.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	c := 0
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {

		switch item.Type {
		case db_models.AdjFund:
			c += 1
		}

		return nil
	})

	assert.Equal(t, 1, c)
	assert.Nil(t, err)
	// assert.Contains(t, err.Error(), "stack nil, mungkin xls bermasalah coba download dengan rentan waktu lebih panjang")
}

func TestTiktokWdContainFailed(t *testing.T) {

	// fname := "../../test/assets/tiktokincome.xlsx"
	fname := "../test/assets/testwd/tiktok_contain_failed.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	c := 0
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {

		switch item.Type {
		case db_models.AdjFund:
			c += 1
		}

		return nil
	})

	assert.Equal(t, 1, c)

	assert.Nil(t, err)
}

func TestTiktokWdFormatBaru(t *testing.T) {

	fname := "../test/assets/testwd/tiktok_format_baru_20_02.xlsx"
	// fname := "../../test/assets/testwd/tiktok_format_baru.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	c := 0
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1
		switch item.Type {
		case db_models.AdjOrderFund:

			assert.NotEqual(t, "0", item.ExternalOrderID)
			assert.GreaterOrEqual(t, 18, len(item.ExternalOrderID))
		}

		return nil
	})

	// assert.Equal(t, 151, c)

	assert.Nil(t, err)
}

func TestTiktokWd(t *testing.T) {

	// fname := "../../test/assets/tiktokincome.xlsx"
	fname := "../test/assets/testwd/tiktok_pzen.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	c := 0
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1
		// debugtool.LogJson(item)
		return nil
	})

	// assert.Equal(t, 151, c)

	assert.Nil(t, err)
}

func TestWithdrawalMap(t *testing.T) {
	mapper := datasource.NewWithdrawalMap()
	t.Run("test dapet earning dulu terus withdrawal dengan time sama", func(t *testing.T) {
		timekeylast := time.Now()

		mapper.Add(&datasource.TiktokDayWDItem{
			Type:        datasource.TiktokWDRecordEarning,
			Amount:      12000,
			RequestTime: timekeylast,
			SuccessTime: timekeylast,
		})

		mapper.Add(&datasource.TiktokDayWDItem{
			Type:        datasource.TiktokWDWithdrawal,
			Amount:      12000,
			RequestTime: timekeylast,
			SuccessTime: timekeylast,
		})

		mapper.Add(&datasource.TiktokDayWDItem{
			Type:        datasource.TiktokWDRecordEarning,
			Amount:      3000,
			RequestTime: timekeylast,
			SuccessTime: timekeylast,
		})

		timekey2 := time.Now().AddDate(0, 0, -1)
		mapper.Add(&datasource.TiktokDayWDItem{
			Type:        datasource.TiktokWDRecordEarning,
			Amount:      9000,
			RequestTime: timekey2,
			SuccessTime: timekey2,
		})

		mapper.Add(&datasource.TiktokDayWDItem{
			Type:        datasource.TiktokWDWithdrawal,
			Amount:      13000,
			RequestTime: timekey2,
			SuccessTime: timekey2,
		})

		t.Run("testing earning pertama", func(t *testing.T) {
			isEmit, wd, err := mapper.WdEmitted(&datasource.TiktokWdItem{
				OrderSettledTime: timekeylast,
				SettlementAmount: 12000,
			})

			assert.Nil(t, err)
			assert.False(t, isEmit)
			assert.Nil(t, wd)
		})

		t.Run("testing untuk earning urutan kedua", func(t *testing.T) {
			isEmit, wd, err := mapper.WdEmitted(&datasource.TiktokWdItem{
				OrderSettledTime: timekeylast,
				SettlementAmount: 3000,
			})

			assert.Nil(t, err)
			assert.True(t, isEmit)
			assert.Equal(t, float64(12000), wd.Amount)
		})

	})
}
func TestTiktokWdDiproses(t *testing.T) {
	fname := "../test/assets/testwd/tiktokwd_process.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		return nil
	})

	assert.ErrorIs(t, err, datasource.ErrContainInProcessWD)
}

func TestTiktokWdStream(t *testing.T) {

	// fname := "../../test/assets/tiktokincome.xlsx"
	// fname := "../../test/assets/testwd/pzen_tidak_complete.xlsx"
	fname := "../test/assets/testwd/tiktok_tidak_selesai.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	c := 0
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1
		// t.Log(item.ExternalOrderID, item.Type, c)
		return nil
	})

	assert.Equal(t, 31, c)

	assert.Nil(t, err)
}

func TestTiktokWdUrutanNgawur(t *testing.T) {

	// fname := "../../test/assets/tiktokincome.xlsx"
	// fname := "../../test/assets/testwd/pzen_tidak_complete.xlsx"
	fname := "../test/assets/testwd/urutan_ngawur.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	c := 0
	amount := 0.00
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1
		amount += item.Amount
		// t.Log(item.ExternalOrderID, item.Type, c)
		return nil
	})

	assert.Equal(t, 19, c)
	assert.Equal(t, 0.00, amount)
	assert.Nil(t, err)
}

func TestTiktokWdTidakValidAtasSendiri(t *testing.T) {

	// fname := "../../test/assets/tiktokincome.xlsx"
	// fname := "../../test/assets/testwd/pzen_tidak_complete.xlsx"
	fname := "../test/assets/testwd/tiktok14-3.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	datas := []*db_models.InvoItem{}
	importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		datas = append(datas, item)

		return nil
	})

	slices.Reverse(datas)

	c := 0
	amount := 0.00
	for _, item := range datas {
		amount += item.Amount
		t.Logf("%s\t\t%2.f\t%2.f --%s", item.Type, item.Amount, amount, item.TransactionDate.String())
		// t.Log(item.Type, item.Amount, amount, item.TransactionDate.String())
		c += 1
	}

	assert.Equal(t, -121745.00, amount)
	assert.Equal(t, 18, c)

	assert.Nil(t, err)
}

func TestStackError(t *testing.T) {
	fname := "../test/assets/testwd/stackerror.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	datas := []*db_models.InvoItem{}
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		datas = append(datas, item)
		assert.NotEqual(t, "0", item.ExternalOrderID)

		return nil
	})
	assert.Nil(t, err)
}

func TestStackErrorSetel(t *testing.T) {
	fname := "../test/assets/testwd/stackerror_setelood.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	datas := []*db_models.InvoItem{}

	ordercount := 0
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		datas = append(datas, item)

		switch item.Type {
		case db_models.AdjOrderFund:
			ordercount += 1

		}

		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, 35, ordercount)
}

func TestSelisihExcel(t *testing.T) {
	fname := "../test/assets/testwd/tiktokselisihpenarikan.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewTiktokWdXls(file)
	datas := []*db_models.InvoItem{}

	wdc := 0
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		datas = append(datas, item)

		switch item.Type {
		case db_models.AdjFund:
			switch wdc {
			case 0:
				assert.Equal(t, 0.00, item.BalanceAfter, item.Amount)
			case 1:
				assert.Equal(t, 40.00, item.BalanceAfter, item.Amount)
			case 2:
				assert.Equal(t, 0.00, item.BalanceAfter, item.Amount)
			case 3:
				assert.Equal(t, 0.00, item.BalanceAfter, item.Amount)
			case 4:
				assert.Equal(t, 2581802.00, item.BalanceAfter, item.Amount)
			default:
				t.Log(item.BalanceAfter, item.Amount)
				assert.Equal(t, 0.00, item.BalanceAfter)

			}

			wdc += 1

		}

		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, 5, wdc)
}
