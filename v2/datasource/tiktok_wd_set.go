package datasource

import (
	"errors"
	"fmt"
	"math"
)

type Earning struct {
	Earning  *TiktokDayWDItem
	Involist InvoItemList
}

type EarningList []*Earning

func (e EarningList) GetAmount() float64 {
	hasil := 0.00
	for _, earning := range e {
		invoAmount := earning.Involist.GetAmount()
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

type WdSet struct {
	Withdrawal  *TiktokDayWDItem
	WdSetBefore *WdSet
	WdSetNext   *WdSet
	Earning     EarningList
	IsLast      bool
}

func (wd *WdSet) FundedEarning() (EarningList, EarningList, error) {
	var err error

	earninglist := EarningList{}
	notfundedlist := EarningList{}

	i := len(wd.Earning)
	var fundedAmount float64 // dont use this for checking further
	wdAmount := math.Abs(wd.Withdrawal.Amount)

	for i > 0 {
		i--
		earning := wd.Earning[i]
		fundedAmount += earning.Earning.Amount
		if fundedAmount <= wdAmount {
			earninglist = append(earninglist, earning)
		} else {
			notfundedlist = append(notfundedlist, earning)
		}
	}

	if earninglist.GetAmount() != wdAmount {
		if len(wd.Earning) == len(earninglist) {
			if wd.WdSetBefore != nil {
				before := wd.WdSetBefore

				_, beforeNotFunded, err := before.FundedEarning()

				if err != nil {
					return earninglist, notfundedlist, err
				}

				earninglist = append(beforeNotFunded, earninglist...)
				if earninglist.GetAmount() != wdAmount {
					return earninglist, notfundedlist, wd.WithErr(fmt.Errorf("cannot trace funded earning wd %.3f and earn %.3f", wdAmount, earninglist.GetAmount()))
				}

				return earninglist, notfundedlist, nil

			} else {
				if !wd.IsLast {
					earn := wd.Earning[len(wd.Earning)-1]
					err = fmt.Errorf("butuh range lebih lama dari %s", earn.Earning.RequestTime.String())
					return earninglist, notfundedlist, err
				}
			}
		}

		return earninglist, notfundedlist, wd.WithErr(fmt.Errorf("cannot trace funded earning wd %.3f and earn %.3f", wdAmount, earninglist.GetAmount()))
	}

	return earninglist, notfundedlist, nil
}

func (w *WdSet) WithErrf(format string, a ...any) error {
	return w.WithErr(fmt.Errorf(format, a...))
}

func (w *WdSet) WithErr(err error) error {
	return fmt.Errorf(
		"withdrawal %.3f at %s error %s",
		w.Withdrawal.Amount,
		w.Withdrawal.SuccessTime,
		err.Error(),
	)
}

// func (wd *WdSet) TraceValidEarning() (EarningList, error) {
// 	result := EarningList{}
// 	result = append(result, wd.Earning...)
// 	notFundedAmount := wd.NotFundedAmount()

// 	if notFundedAmount < 0 {

// 		before := wd.WdSetBefore
// 		if before == nil {
// 			return nil, wd.WithErr(errors.New("before not found"))
// 		}

// 		// log.Println("getting fund", before.Withdrawal.Amount, len(before.Earning))
// 		_, notfunded, err := before.FundedEarning()
// 		if err != nil {
// 			return result, err
// 		}

// 		// log.Println(before.)
// 		// log.Println(notFundedAmount, notfunded.GetAmount())

// 		if notfunded.GetAmount() == math.Abs(notFundedAmount) {
// 			result = append(result, notfunded...)
// 		}

// 	}

// 	if notFundedAmount > 0 {

// 	}

// 	return result, nil
// }

func (wd *WdSet) NotFundedAmount() float64 {
	return wd.Earning.GetAmount() - math.Abs(wd.Withdrawal.Amount)
}

// func (wd *WdSet) FundedEarning() (EarningList, EarningList, error) {
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
