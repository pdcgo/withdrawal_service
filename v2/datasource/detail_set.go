package datasource

import (
	"math"
	"time"

	"github.com/pdcgo/shared/db_models"
)

type DetailSet struct {
	Data   []*db_models.InvoItem
	getted map[string]bool
}

func (ds *DetailSet) GetOrderEarning(txDate time.Time) (InvoItemList, error) {
	var err error
	result := InvoItemList{}
	for _, invo := range ds.Data {
		if invo.TransactionDate.Unix() == txDate.Unix() {
			if invo.Type != db_models.AdsPayment {
				result = append(result, invo)
			}
		}
	}

	return result, err
}

func (ds *DetailSet) GetGmvDeduction(txDate time.Time, amount float64) (InvoItemList, error) {
	var err error

	result := InvoItemList{}
	amount = math.Abs(amount)
	var iterAmount float64

	// log.Println("-----------------------------------")
	for _, invo := range ds.Data {

		if invo.TransactionDate.Unix() != txDate.Unix() {
			continue
		}

		if invo.Type != db_models.AdsPayment {
			continue
		}

		// debugtool.LogJson(amount, invo)

		if ds.getted[invo.ExternalOrderID] {
			continue
		}

		iterAmount += math.Abs(invo.Amount)
		// log.Printf("iter: %.3f amount: %.3f need: %.3f\n", iterAmount, math.Abs(invo.Amount), amount)
		if iterAmount <= amount {
			result = append(result, invo)
			ds.getted[invo.ExternalOrderID] = true
			continue
		} else {
			break
		}
	}

	return result, err
}
