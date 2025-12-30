package datasource_shopee

import (
	"context"
	"sort"

	"github.com/pdcgo/shared/db_models"
)

type InvoItemList []*db_models.InvoItem

func (list InvoItemList) Sort() {
	sort.Slice(list, func(i, j int) bool {
		return list[i].TransactionDate.UnixMilli() > list[j].TransactionDate.UnixMilli()
	})
}

func (list InvoItemList) Withdrawals(ctx context.Context) ([]*ShopeeWdSet, error) {

	wds := []*ShopeeWdSet{}

	var wd *ShopeeWdSet

Parent:
	for _, item := range list {
		switch item.Type {
		case db_models.AdjFund:
			oldwd := wd
			wd = &ShopeeWdSet{
				Withdrawal: item,
				Earning:    EarningList{},
				WdSetNext:  oldwd,
			}

			if oldwd != nil {
				oldwd.WdSetBefore = wd
			}

			wds = append(wds, wd)

		default:
			if wd == nil {
				continue Parent
			}
			wd.Earning = append(wd.Earning, item)
		}
	}

	// setting last
	if wd != nil {
		wd.IsLast = true
	}

	return wds, nil
}
