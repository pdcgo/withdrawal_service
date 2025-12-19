package datasource_test

import (
	"context"
	"os"
	"testing"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/withdrawal_service/datasource"
	"github.com/stretchr/testify/assert"
)

func TestDatasourceShopeeWdExcelKlipsa(t *testing.T) {
	fname := "../test/assets/klipsashopee.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeWdXls(file)
	c := 0
	importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		switch item.Type {
		case db_models.AdjFund:
			c += 1

		}
		return nil
	})

	assert.Equal(t, 18, c)

}

func TestDatasourceShopeeWdExcel(t *testing.T) {
	fname := "../test/assets/rosapea.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeWdXls(file)
	c := 0
	importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1
		return nil
	})

	assert.Equal(t, 7, c)

	t.Run("getting username", func(t *testing.T) {
		username, err := importer.GetShopUsername()
		assert.NotEmpty(t, username)
		assert.Nil(t, err)
		assert.Equal(t, "rosapeastyle", username)
	})

}

func TestDatasourceShopeeWdExcelDuplicateFund(t *testing.T) {
	fname := "../test/assets/shopee_invoice_adj.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeWdXls(file)
	c := 0
	importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		// log.Println(item.Amount)
		switch item.Type {
		case db_models.AdjFund:
			c += 1
		}

		return nil
	})

	assert.Equal(t, 11, c)

}

func TestDatasourceShopeeWdExcelPenyesuaian(t *testing.T) {
	fname := "../test/assets/testwd/penyesuaian.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeWdXls(file)
	c := 0
	importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1
		if item.ExternalOrderID == "241029SNV1JB5K" && item.Amount == 5000.00 {
			assert.Equal(t, db_models.AdjPackaging, item.Type)
		}

		if item.ExternalOrderID == "241106G3CQC53J" && item.Amount == 5000.00 {
			assert.Equal(t, db_models.AdjUnknownAdj, item.Type)
		}

		return nil
	})

	assert.Equal(t, 656, c)

}

func TestDatasourceShopeeWdExcelOrderIDDesc(t *testing.T) {
	fname := "../test/assets/testwd/orderid_at_desc.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeWdXls(file)
	c := 0
	importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1
		if item.Amount == -618.00 {
			assert.NotEqual(t, "-", item.ExternalOrderID)
			assert.NotEqual(t, "", item.ExternalOrderID)
			// t.Error(item.Type, item.ExternalOrderID)
		}
		return nil
	})

	assert.Equal(t, 18, c)

}

func TestDatasourceShopeeWdExcelKukiTidakCompleted(t *testing.T) {
	fname := "../test/assets/testwd/kukitidakcompleted.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeWdXls(file)
	c := 0
	importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1

		return nil
	})

	assert.Equal(t, 78, c)

}

func TestDatasourceShopeeWdInprocessWithdrawal(t *testing.T) {
	fname := "../test/assets/testwd/shopee_penarikan_process.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeWdXls(file)
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		return nil
	})

	assert.ErrorIs(t, err, datasource.ErrContainInProcessWD)

}

func TestDatasourceShopeeWdGagalWithdrawal(t *testing.T) {
	fname := "../test/assets/testwd/shopee_penarikan_gagal.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeWdXls(file)
	c := 0
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, 6, c)

}

func TestShopeeOrderTimeNull(t *testing.T) {
	fname := "../test/assets/testwd/shopee_wd_ordertimenull.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewShopeeWdXls(file)
	c := 0
	err = importer.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		c += 1
		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, 3, c)
}
