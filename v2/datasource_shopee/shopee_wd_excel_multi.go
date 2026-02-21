package datasource_shopee

import (
	"context"
	"fmt"
	"io"
	"math"
	"time"

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

	df := NewInvoListDataframe(unordered)

	invWds := df.
		Query(
			df.D.Type.Filter(func(i int, item db_models.AdjustmentType) bool {
				return item == db_models.AdjFund
			}),
			df.D.Amount.Filter(func(i int, item float64) bool {
				return item < 0
			}),
			df.D.Failed.Filter(func(i int, item bool) bool {
				return item == false
			}),
		).
		Data()

	// sumover value
	sumAmountDf := Series[float64]{}
	sumamount := 0.0
	for i := len(df.D.Amount) - 1; i >= 0; i-- {
		sumamount += df.D.Amount[i]
		sumAmountDf = append(sumAmountDf, sumamount)
	}

	result := []*ShopeeWdSet{}
	notFundMap := map[int]EarningList{}

	for i, invWd := range invWds {
		var notFund EarningList
		// getting not funded
		notFund = df.
			Query(
				df.D.TransactionDate.Filter(func(i int, item time.Time) bool {
					return item.Before(invWd.TransactionDate)
				}),
				df.D.BalanceAfter.Break(false, func(i int, item float64) bool {
					return item == math.Abs(invWd.Amount)
				}),
			).Data()

		if notFund.GetAmount() != invWd.BalanceAfter {
			return result, fmt.Errorf("not funded is %.1f but balance after is %.1f", notFund.GetAmount(), invWd.BalanceAfter)
		}

		if len(notFund) == 0 {
			continue
		}

		notFundMap[i] = notFund

		// result = append(result, &ShopeeWdSet{
		// 	Withdrawal: invWd,
		// 	Earning:    notFund,
		// 	IsLast:     true,
		// })
	}

	for i, invWd := range invWds {
		var fund EarningList
		fundf := df.
			Query(
				df.D.TransactionDate.Filter(func(i int, item time.Time) bool {
					return item.Before(invWd.TransactionDate)
				}),
			)

		notFund, ok := notFundMap[i]
		if !ok {

			fundf = fundf.
				Query(
					fundf.D.BalanceAfter.Break(false, func(i int, item float64) bool {

						// fmt.Printf("%d %.1f\n", i, item)
						return item == 0
					}),
				)
		} else {
			first := notFund[len(notFund)-1]

			cc := 0.0

			fundf = fundf.
				Query(
					fundf.D.TransactionDate.Filter(func(i int, item time.Time) bool {
						return item.Before(first.TransactionDate)
					}),

					fundf.D.Amount.Break(true, func(i int, item float64) bool {
						cc += item
						return cc == math.Abs(invWd.Amount)
					}),

					// fundf.D.Amount.SearchPosition(func(partial Series[float64]) (bool, bool) {
					// 	res := 0.0
					// 	for _, item := range partial {
					// 		res += item
					// 	}
					// 	return res > math.Abs(invWd.Amount), res == math.Abs(invWd.Amount)
					// }),
				)

			// debugtool.LogJson(fundf.Data())
		}
		fund = fundf.Data()

		if math.Abs(invWd.Amount) != fund.GetAmount() {
			if i == (len(invWds)-1) && i > 1 {
				return result, nil
			}

			return result, fmt.Errorf("funded is %.1f but withdrawal is %.1f", fund.GetAmount(), math.Abs(invWd.Amount))
		}

		result = append(result, &ShopeeWdSet{
			Withdrawal: invWd,
			Earning:    fund,
		})

	}

	// // bagian lama

	// wds, err := unordered.Withdrawals(ctx)
	// if err != nil {
	// 	return wds, err
	// }

	// // mulai loop untuk get valid wd
	// result := []*ShopeeWdSet{}

	// for _, wd := range wds {
	// 	notFundedAmount := wd.NotFundedAmount()

	// 	if notFundedAmount != 0 {
	// 		validEarning, err := wd.TraceValidEarning()
	// 		if err != nil {

	// 			if wd.IsLast {
	// 				return result, nil
	// 			}

	// 			return result, err
	// 		}

	// 		result = append(result, &ShopeeWdSet{
	// 			Withdrawal:  wd.Withdrawal,
	// 			WdSetBefore: wd.WdSetBefore,
	// 			WdSetNext:   wd.WdSetNext,
	// 			Earning:     validEarning,
	// 			IsLast:      wd.IsLast,
	// 		})
	// 	}

	// 	// if notFundedAmount > 0 {
	// 	// 	debugtool.LogJson(wd)
	// 	// }

	// 	if notFundedAmount == 0 {
	// 		result = append(result, wd)
	// 	}
	// }

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
