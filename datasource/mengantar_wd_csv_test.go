package datasource_test

import (
	"context"
	"os"
	"testing"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/withdrawal_service/datasource"
	"github.com/stretchr/testify/assert"
)

func TestIterateMengantar(t *testing.T) {
	fname := "../test/assets/mengantar/sample.csv"
	file, err := os.Open(fname)
	assert.Nil(t, err)
	defer file.Close()

	mengan := datasource.NewMengantarWdCsv(file)
	refList, err := mengan.GetRefIDs()

	assert.Nil(t, err)
	assert.Len(t, refList, 3)

	var amount float64 = 0
	wdCount := 0
	err = mengan.Iterate(context.Background(), func(item *db_models.InvoItem) error {
		switch item.Type {
		case db_models.AdjFund:
			wdCount += 1
			// assert.Equal(t, 471826.2302, item.Amount)

		case db_models.AdjOrderFund:
			amount += item.Amount
		}

		return nil
	})
	assert.Nil(t, err)
	assert.Equal(t, 3, wdCount)
	assert.Equal(t, 471826.2302, amount)
}
