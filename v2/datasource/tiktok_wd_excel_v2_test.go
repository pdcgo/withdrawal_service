package datasource_test

import (
	"os"
	"testing"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/withdrawal_service/v2/datasource"
	"github.com/stretchr/testify/assert"
)

func TestTiktokDatasource(t *testing.T) {
	fname := "../../test/assets/pzen_complete_salah.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewV2TiktokWdXls(file)
	_, err = importer.IterateWithdrawal()
	assert.Nil(t, err)

	// for _, wd := range wds {
	// 	assert.Equal(t, math.Abs(wd.Withdrawal.Amount), wd.Earning.GetAmount())
	// 	debugtool.LogJson(wd)
	// }

}

func TestOrderDatasource(t *testing.T) {
	fname := "../../test/assets/tiktok/87_IDLCAHTLWX6_10_05_2025_12_08.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewV2TiktokWdXls(file)
	wds, err := importer.IterateValidWithdrawal()
	assert.Nil(t, err)

	for _, wd := range wds {
		for _, earn := range wd.Earning {
			for _, invo := range earn.Involist {
				switch invo.Amount {
				case 148062:
					assert.Equal(t, "581451640366466313", invo.ExternalOrderID)
					assert.Equal(t, db_models.AdjOrderFund, invo.Type)
				}
			}

		}

	}
}

func TestTiktokGmvPayment(t *testing.T) {
	fname := "../../test/assets/tiktok/gmv_payment.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewV2TiktokWdXls(file)
	wds, err := importer.IterateWithdrawal()
	assert.Nil(t, err)

	ads := 0

	for _, wd := range wds {
		for _, earn := range wd.Earning {
			for _, invo := range earn.Involist {
				switch invo.Type {
				case db_models.AdsPayment:

					assert.Equal(t, -9246299.00, invo.Amount)
					ads += 1
				}
			}
		}
	}

	assert.Equal(t, 1, ads)
}

func TestAdditionalCampaign(t *testing.T) {
	fname := "../../test/assets/tiktok/husen_campaign.xlsx"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	importer := datasource.NewV2TiktokWdXls(file)
	wds, err := importer.IterateWithdrawal()
	assert.Nil(t, err)

	for _, wd := range wds {
		for _, earn := range wd.Earning {
			for _, invo := range earn.Involist {
				t.Log(invo.Type)
			}
		}
	}
}
