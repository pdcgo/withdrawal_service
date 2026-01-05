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

func (wd *ShopeeWdSet) FundedEarning() (EarningList, error) {
	earnlist := EarningList{}

	var fundedAmount float64
	wdAmount := math.Abs(wd.Withdrawal.Amount)

	if wd.Withdrawal.BalanceAfter < 0 {
		return earnlist, errors.New("balance after minus not implemented")
	}

	i := 0
	var notFundedAmount float64
	if wd.Withdrawal.BalanceAfter > 0 {
		for i < len(wd.Earning) {

			notFundedAmount += wd.Earning[i].Amount
			if notFundedAmount <= wd.Withdrawal.BalanceAfter {
				// fmt.Printf("not funded %.3f - %d - %.3f \n", wd.Withdrawal.Amount, i, wd.Earning[i].Amount)
				// earnlist = append(earnlist, wd.Earning[i])
				i++
			} else {
				break
			}
		}
	}

	// fmt.Printf("funded %.3f sddd \n", fundedAmount)
	c := len(wd.Earning) - 1
	// log.Println("cc", len(wd.Earning), wd.Withdrawal.Amount)
	for c >= i {

		fundedAmount += wd.Earning[c].Amount

		if fundedAmount <= wdAmount {
			// fmt.Printf("funded %.3f - %d - %.3f - %.3f \n", wd.Withdrawal.Amount, c, wd.Earning[c].Amount, fundedAmount)
			// earnlist = append(earnlist, wd.Earning[c])
			earnlist = append([]*db_models.InvoItem{wd.Earning[c]}, earnlist...)
			c--
		} else {
			break
		}

		if fundedAmount == wdAmount {
			break
		}

		// if fundedAmount > wdAmount {
		// 	if wd.Earning[i].Amount < 0 {
		// 		fmt.Printf("dfunded %.3f - %d - %.3f - %.3f \n", wd.Withdrawal.Amount, i, wd.Earning[i].Amount, fundedAmount)
		// 		earnlist = append(earnlist, wd.Earning[i])
		// 		i++
		// 		continue
		// 	} else {
		// 		fmt.Printf("dskip %.3f - %d - %.3f - %.3f \n", wd.Withdrawal.Amount, i, wd.Earning[i].Amount, fundedAmount)
		// 		break
		// 	}

		// }

	}

	if wd.WdSetBefore != nil {
		before := wd.WdSetBefore
		if before.Withdrawal.BalanceAfter > 0 {
			beforeNotFund, err := before.NotFundedEarning()
			if err != nil {
				return earnlist, err
			}

			// for _, earn := range beforeNotFund {
			// 	fmt.Printf("before %.3f - %d - %.3f \n", before.Withdrawal.Amount, i, earn.Amount)
			// }

			earnlist = append(earnlist, beforeNotFund...)

		}
	}

	if wdAmount != earnlist.GetAmount() {
		// debugtool.LogJson(earnlist)
		return earnlist, wd.WithErr(fmt.Errorf("cannot trace funded earning wd %.3f and earn %.3f", wdAmount, earnlist.GetAmount()))
	}

	return earnlist, nil
}

func (wd *ShopeeWdSet) NotFundedEarning() (EarningList, error) {
	earnlist := EarningList{}

	if wd.Withdrawal.BalanceAfter == 0 {
		return earnlist, nil
	}

	if wd.Withdrawal.BalanceAfter < 0 {
		return earnlist, errors.New("balance after minus not implemented")
	}

	if wd.Withdrawal.BalanceAfter > 0 {
		var targetfund float64
		for _, earn := range wd.Earning {
			targetfund += earn.Amount
			if targetfund <= wd.Withdrawal.BalanceAfter {
				earnlist = append(earnlist, earn)
			} else {
				break
			}
		}
	}

	if wd.Withdrawal.BalanceAfter != earnlist.GetAmount() {
		return earnlist, fmt.Errorf("tidak bisa detect sisa balance withdrawal %.3f", wd.Withdrawal.BalanceAfter)
	}

	return earnlist, nil
}

func (wd *ShopeeWdSet) NotFundedAmount() float64 {
	return wd.Earning.GetAmount() - math.Abs(wd.Withdrawal.Amount)
}

func (wd *ShopeeWdSet) TraceValidEarning() (EarningList, error) {
	var err error
	result := EarningList{}
	result = append(result, wd.Earning...)

	notFundedAmount := wd.NotFundedAmount()

	if notFundedAmount < 0 {
		result, err = wd.FundedEarning()
		if err != nil {
			return result, err
		}

		if math.Abs(wd.Withdrawal.Amount) != result.GetAmount() {
			return result, wd.WithErr(fmt.Errorf("cannot trace valid earning %.3f", result.GetAmount()))
		}

	}

	if notFundedAmount > 0 {
		return wd.FundedEarning()
	}

	return result, nil
}

// func (wd *ShopeeWdSet) FundedEarning() (EarningList, EarningList, error) {
// 	var earning EarningList
// 	var notEarning EarningList

// 	wdAbsAmount := math.Abs(wd.Withdrawal.Amount)
// 	err := ComboIndices(len(wd.Earning), func(index []int) error {

// 		dd := wd.Earning.SubsetIndex(index, false)
// 		if dd.GetAmount() == wdAbsAmount {
// 			earning = dd
// 			notEarning = wd.Earning.SubsetIndex(index, true)
// 			return ErrComboStop
// 		}
// 		return nil
// 	})

// 	return earning, notEarning, err
// }

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
