package datasource_shopee

import (
	"time"

	"github.com/pdcgo/shared/db_models"
)

// Deprecated: asasdasdad
type Series[T any] []T
type OffsetFilter func([]int) []int

func (s Series[T]) Filter(handler func(i int, item T) bool) OffsetFilter {
	return func(offset []int) []int {
		result := []int{}
		for _, i := range offset {
			if handler(i, s[i]) {
				result = append(result, i)
			}
		}
		return result
	}
}

func (s Series[T]) Break(includeBreak bool, handler func(i int, item T) bool) OffsetFilter {
	return func(offset []int) []int {
		result := []int{}
		for _, i := range offset {
			if handler(i, s[i]) {
				if includeBreak {
					result = append(result, i)
				}
				break
			}

			result = append(result, i)
		}
		return result
	}
}

func (s Series[T]) BreakReverse(handler func(i int, item T) bool) OffsetFilter {
	return func(offset []int) []int {
		result := []int{}
		for i := len(offset) - 1; i >= 0; i-- {
			if handler(i, s[i]) {
				break
			}

			result = append(result, i)
		}
		return result
	}
}

func (s Series[T]) SearchPosition(handler func(partial Series[T]) (bool, bool)) OffsetFilter {

	var getpartial = func(ofset []int) Series[T] {
		partial := Series[T]{}
		for _, i := range ofset {
			partial = append(partial, s[i])
		}
		return partial
	}

	return func(ofset []int) []int {

		var pos int = len(ofset)

		c := 0
		csize := len(ofset)

		for c < len(ofset) {
			if len(ofset) == 0 {
				return []int{}
			}

			prev, breakIt := handler(getpartial(ofset[:pos]))
			if breakIt {
				break
			}

			var newpos int
			if csize > 1 {
				csize = csize / 2
			}

			if prev {
				newpos = pos - csize

			} else {
				newpos = pos + csize

			}

			pos = newpos
			// fmt.Printf("pos %d new pos %d\n", pos, newpos)

			c++
		}

		return ofset[:pos]
	}
}

// func (s Series[T]) Filter(handler func(item T) bool) FilterOffset {
// 	result := FilterOffset{}
// 	for i, item := range s {
// 		if handler(item) {
// 			result = append(result, i)
// 		}
// 	}
// 	return result
// }

type dataframeData struct {
	ExternalOrderID Series[string]
	Type            Series[db_models.AdjustmentType]
	TransactionDate Series[time.Time]
	Description     Series[string]
	Amount          Series[float64]
	BalanceAfter    Series[float64]
	Region          Series[string]
	IsOtherRegion   Series[bool]
	Failed          Series[bool]
}

type InvoListDataframe struct {
	count  int
	offset []int
	D      *dataframeData
}

func NewInvoListDataframe(datas []*db_models.InvoItem) *InvoListDataframe {
	d := &InvoListDataframe{
		count:  0,
		offset: []int{},
		D: &dataframeData{
			ExternalOrderID: Series[string]{},
			Type:            Series[db_models.AdjustmentType]{},
			TransactionDate: Series[time.Time]{},
			Description:     Series[string]{},
			Amount:          Series[float64]{},
			BalanceAfter:    Series[float64]{},
			Region:          Series[string]{},
			IsOtherRegion:   Series[bool]{},
			Failed:          Series[bool]{},
		},
	}

	for _, item := range datas {
		d.D.ExternalOrderID = append(d.D.ExternalOrderID, item.ExternalOrderID)
		d.D.Type = append(d.D.Type, item.Type)
		d.D.TransactionDate = append(d.D.TransactionDate, item.TransactionDate)
		d.D.Description = append(d.D.Description, item.Description)
		d.D.Amount = append(d.D.Amount, item.Amount)
		d.D.BalanceAfter = append(d.D.BalanceAfter, item.BalanceAfter)
		d.D.Region = append(d.D.Region, item.Region)
		d.D.IsOtherRegion = append(d.D.IsOtherRegion, item.IsOtherRegion)
		d.D.Failed = append(d.D.Failed, item.Failed)
		d.offset = append(d.offset, d.count)
		d.count++
	}

	return d

}

func (d *InvoListDataframe) Query(filters ...OffsetFilter) *InvoListDataframe {

	offset := d.offset
	for _, filter := range filters {
		offset = filter(offset)

	}

	newdf := InvoListDataframe{
		count:  len(offset),
		D:      d.D,
		offset: offset,
	}

	return &newdf
}

func (d *InvoListDataframe) First() *db_models.InvoItem {
	if len(d.offset) == 0 {
		return nil
	}

	i := d.offset[0]
	return d.get(i)
}

func (d *InvoListDataframe) get(i int) *db_models.InvoItem {
	item := db_models.InvoItem{
		ExternalOrderID: d.D.ExternalOrderID[i],
		Type:            d.D.Type[i],
		TransactionDate: d.D.TransactionDate[i],
		Description:     d.D.Description[i],
		Amount:          d.D.Amount[i],
		BalanceAfter:    d.D.BalanceAfter[i],
		Region:          d.D.Region[i],
		IsOtherRegion:   d.D.IsOtherRegion[i],
		Failed:          d.D.Failed[i],
	}

	return &item
}

func (d *InvoListDataframe) Last() *db_models.InvoItem {
	if len(d.offset) == 0 {
		return nil
	}

	i := d.offset[len(d.offset)-1]
	return d.get(i)

}

func (d *InvoListDataframe) Data() []*db_models.InvoItem {
	result := []*db_models.InvoItem{}
	for _, i := range d.offset {
		result = append(result, d.get(i))
	}

	return result
}
