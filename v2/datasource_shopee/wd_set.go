package datasource_shopee

import (
	"errors"
	"fmt"
	"math"

	"github.com/pdcgo/shared/db_models"
)

type ShopeeWdSet struct {
	Withdrawal  *db_models.InvoItem
	WdSetBefore *ShopeeWdSet
	WdSetNext   *ShopeeWdSet
	Earning     EarningList
	IsLast      bool
}

func (w *ShopeeWdSet) WithErr(err error) error {
	return fmt.Errorf(
		"withdrawal %.3f at %s error %s",
		w.Withdrawal.Amount,
		w.Withdrawal.TransactionDate,
		err.Error(),
	)
}

func (wd *ShopeeWdSet) NotFundedAmount() float64 {
	return wd.Earning.GetAmount() - math.Abs(wd.Withdrawal.Amount)
}

func (wd *ShopeeWdSet) TraceValidEarning() (EarningList, error) {
	result := EarningList{}
	result = append(result, wd.Earning...)

	notFundedAmount := wd.NotFundedAmount()

	if notFundedAmount < 0 {
		before := wd.WdSetBefore
		if before == nil {
			return nil, wd.WithErr(errors.New("before not found"))
		}

		notFundedAmountAbs := math.Abs(notFundedAmount)

		var beforeFundedAmount float64
		beforeEarn := EarningList{}

		for _, earning := range before.Earning {
			beforeFundedAmount += earning.Amount
			if beforeFundedAmount <= notFundedAmountAbs {
				beforeEarn = append(beforeEarn, earning)
			} else {
				break
			}
		}

		result = append(result, beforeEarn...)

		if math.Abs(wd.Withdrawal.Amount) != result.GetAmount() {
			return result, wd.WithErr(fmt.Errorf("cannot trace valid earning %.3f", result.GetAmount()))
		}

	}

	if notFundedAmount > 0 {
		var beforeFundedAmount float64
		beforeEarn := EarningList{}

		fundedAmountAbs := math.Abs(wd.Withdrawal.Amount)

		var c int = len(wd.Earning) - 1
		for c >= 0 {
			beforeFundedAmount += wd.Earning[c].Amount
			if beforeFundedAmount <= fundedAmountAbs {
				beforeEarn = append(beforeEarn, wd.Earning[c])
			} else {
				break
			}
			c--
		}

		result = beforeEarn

		if math.Abs(wd.Withdrawal.Amount) != result.GetAmount() {
			return result, wd.WithErr(fmt.Errorf("cannot trace valid earning %.3f", result.GetAmount()))
		}
	}

	return result, nil
}

func (wd *ShopeeWdSet) FundedEarning() (EarningList, EarningList, error) {
	var earning EarningList
	var notEarning EarningList

	wdAbsAmount := math.Abs(wd.Withdrawal.Amount)
	err := ComboIndices(len(wd.Earning), func(index []int) error {

		dd := wd.Earning.SubsetIndex(index, false)
		if dd.GetAmount() == wdAbsAmount {
			earning = dd
			notEarning = wd.Earning.SubsetIndex(index, true)
			return ErrComboStop
		}
		return nil
	})

	return earning, notEarning, err
}

type EarningList []*db_models.InvoItem

func (e EarningList) GetAmount() float64 {
	hasil := 0.00
	for _, earning := range e {
		invoAmount := earning.Amount
		hasil += invoAmount
	}
	return hasil
}

func (e EarningList) SubsetIndex(index []int, reverse bool) EarningList {
	res := EarningList{}

	var newindex []int

	if reverse {
		newindex = []int{}
		for i := range e {
			// log.Println(index, i, Contains(index, i))
			if Contains(index, i) {
				continue
			}
			newindex = append(newindex, i)
		}

		// log.Println("asdasd", newindex, index)

	} else {
		newindex = index
	}

	for _, i := range newindex {
		res = append(res, e[i])
	}
	return res
}

type OrderRefList []string

func (d *OrderRefList) Add(str string) {
	switch str {
	case "", "-":
		return
	default:
		*d = append(*d, str)
	}
}

var ErrComboStop error = errors.New("combo stop")

// AllComboIndices generates all combinations for lengths 1..n.
// For n items, it yields: C(n,1), C(n,2), ..., C(n,n)
func ComboIndices(n int, handler func(datas []int) error) error {

	if n <= 0 {
		return nil
	}

	// Generate combinations of length k
	// var gen func(k int)
	gen := func(k int) error {
		comb := make([]int, k)

		var backtrack func(start, depth int) error
		backtrack = func(start, depth int) error {
			if depth == k {
				tmp := make([]int, k)
				copy(tmp, comb)

				return handler(tmp)
			}

			for i := start; i <= n-(k-depth); i++ {
				comb[depth] = i
				err := backtrack(i+1, depth+1)
				if err != nil {
					return err
				}
			}

			return nil
		}

		return backtrack(0, 0)
	}

	// Loop k = 1..n
	for k := 1; k <= n; k++ {
		err := gen(k)
		if err != nil {
			if errors.Is(err, ErrComboStop) {
				return nil
			}
			return err
		}
	}

	return nil
}

func Contains[T comparable](slice []T, v T) bool {
	for _, x := range slice {
		if x == v {
			return true
		}
	}
	return false
}
