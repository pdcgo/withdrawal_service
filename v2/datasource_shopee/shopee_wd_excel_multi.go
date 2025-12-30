package datasource_shopee

import (
	"context"
	"io"

	"github.com/pdcgo/shared/db_models"
)

type wdMultiFileImpl struct {
	files []*shopeeXlsImpl
}

// GetRefIDs implements withdrawal.Source.
func (w *wdMultiFileImpl) GetRefIDs() (OrderRefList, error) {
	var err error

	result := OrderRefList{}
	for _, file := range w.files {
		refids, err := file.GetRefIDs()
		if err != nil {
			return result, err
		}

		result = append(result, refids...)
	}

	return result, err

}

// ValidWithdrawal implements withdrawal.Source.
func (w *wdMultiFileImpl) ValidWithdrawal(ctx context.Context) ([]*ShopeeWdSet, error) {
	var err error
	unordered := InvoItemList{}
	for _, file := range w.files {
		err = file.Iterate(ctx, func(item *db_models.InvoItem) error {
			unordered = append(unordered, item)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	unordered.Sort()
	wds, err := unordered.Withdrawals(ctx)
	if err != nil {
		return wds, err
	}

	// mulai loop untuk get valid wd
	result := []*ShopeeWdSet{}

	for _, wd := range wds {
		notFundedAmount := wd.NotFundedAmount()

		if notFundedAmount != 0 {
			validEarning, err := wd.TraceValidEarning()
			if err != nil {

				if wd.IsLast {
					return result, nil
				}

				return result, err
			}

			result = append(result, &ShopeeWdSet{
				Withdrawal:  wd.Withdrawal,
				WdSetBefore: wd.WdSetBefore,
				WdSetNext:   wd.WdSetNext,
				Earning:     validEarning,
				IsLast:      wd.IsLast,
			})
		}

		// if notFundedAmount > 0 {
		// 	debugtool.LogJson(wd)
		// }

		if notFundedAmount == 0 {
			result = append(result, wd)
		}
	}

	return result, nil

}

// GetShopUsername implements withdrawal.Source.
func (w *wdMultiFileImpl) GetShopUsername() (string, error) {
	var err error

	var username string
	for _, file := range w.files {
		usr, err := file.GetShopUsername()
		if err != nil {
			return username, err
		}

		if username != "" {
			if username != usr {
				return username, ErrCannotGetMarketplaceUsername
			}
		}

		username = usr

	}

	return username, err
}

func NewShopeeXlsMultiFile(readers []io.ReadCloser) (*wdMultiFileImpl, error) {
	var err error
	files := []*shopeeXlsImpl{}

	for _, reader := range readers {
		files = append(files, NewShopeeXlsWithdrawal(reader))
	}

	return &wdMultiFileImpl{
		files,
	}, err
}
