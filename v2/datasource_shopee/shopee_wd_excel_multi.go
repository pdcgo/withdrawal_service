package datasource_shopee

import (
	"context"
	"fmt"
	"io"
	"math"
	"strings"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/common_helper"
)

//go:generate go run github.com/wargasipil/data_processing
type Withdrawal struct {
	Withdrawal *db_models.InvoItem
	List       EarningList
}

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
	var result []*ShopeeWdSet = []*ShopeeWdSet{}
	failedMap := map[float64]EarningList{}

	caller := common_helper.NewChainParam[*db_models.InvoItemDataFrame](
		func(next common_helper.NextFuncParam[*db_models.InvoItemDataFrame]) common_helper.NextFuncParam[*db_models.InvoItemDataFrame] {
			return func(data *db_models.InvoItemDataFrame) (*db_models.InvoItemDataFrame, error) { // creating df
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

				df := db_models.NewInvoItemDataFrame(unordered)
				return next(df)
			}
		},
		func(next common_helper.NextFuncParam[*db_models.InvoItemDataFrame]) common_helper.NextFuncParam[*db_models.InvoItemDataFrame] {
			return func(df *db_models.InvoItemDataFrame) (*db_models.InvoItemDataFrame, error) { // creating wd set
				invWds := df.
					Query(
						df.D.Type.Filter(func(i int, item db_models.AdjustmentType) bool {
							return item == db_models.AdjFund
						}),
						df.D.Amount.Filter(func(i int, item float64) bool {
							return item < 0
						}),
						// df.D.Failed.Filter(func(i int, item bool) bool {
						// 	return item == false
						// }),
					).
					Data()

				// sumover value
				sumAmountDf := Series[float64]{}
				sumamount := 0.0
				for i := len(df.D.Amount) - 1; i >= 0; i-- {
					sumamount += df.D.Amount[i]
					sumAmountDf = append(sumAmountDf, sumamount)
				}

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
						return nil, fmt.Errorf("not funded is %.1f but balance after is %.1f", notFund.GetAmount(), invWd.BalanceAfter)
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

				wdlen := len(invWds)

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
						if i == (wdlen-1) && wdlen > 1 {
							return next(df)
						}

						return nil, fmt.Errorf("funded is %.1f but withdrawal is %.1f", fund.GetAmount(), math.Abs(invWd.Amount))
					}

					result = append(result, &ShopeeWdSet{
						Withdrawal: invWd,
						Earning:    fund,
					})

				}

				return next(df)
			}
		},
		func(next common_helper.NextFuncParam[*db_models.InvoItemDataFrame]) common_helper.NextFuncParam[*db_models.InvoItemDataFrame] {
			return func(df *db_models.InvoItemDataFrame) (*db_models.InvoItemDataFrame, error) { // check jika ada wd gagal

				// mapping failed
				for _, wd := range result {
					if wd.Withdrawal.Failed {
						key := math.Abs(wd.Withdrawal.Amount)
						failedMap[key] = wd.Earning
					}
				}

				return next(df)
			}
		},

		func(next common_helper.NextFuncParam[*db_models.InvoItemDataFrame]) common_helper.NextFuncParam[*db_models.InvoItemDataFrame] {
			return func(df *db_models.InvoItemDataFrame) (*db_models.InvoItemDataFrame, error) {

				for _, wd := range result {
					// search jika ada yang gagal
					var position int
					var invo *db_models.InvoItem

					for i, item := range wd.Earning {
						if strings.Contains(item.Description, "Pengembalian Dana untuk Penarikan Gagal") {
							position = i
							invo = item
							break
						}
					}

					if position == 0 {
						continue
					}

					fwd, ok := failedMap[invo.Amount]

					if !ok {
						return df, fmt.Errorf("cannot find withdrawal invoice list of %.2f", invo.Amount)
					}

					newearn := EarningList{}
					newearn = append(newearn, wd.Earning[:position]...)
					newearn = append(newearn, fwd...)
					newearn = append(newearn, wd.Earning[position:]...)
					wd.Earning = newearn

				}

				return next(df)
			}

		},
		func(next common_helper.NextFuncParam[*db_models.InvoItemDataFrame]) common_helper.NextFuncParam[*db_models.InvoItemDataFrame] {
			return func(df *db_models.InvoItemDataFrame) (*db_models.InvoItemDataFrame, error) { // finalize wd
				newresult := []*ShopeeWdSet{}
				for _, item := range result {
					if item.Withdrawal.Failed {
						continue
					}

					newresult = append(newresult, item)
				}
				result = newresult

				return next(df)
			}

		},
	)

	_, err = caller(&db_models.InvoItemDataFrame{})
	if err != nil {
		return result, err
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
