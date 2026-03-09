package datasource_shopee

import (
	"github.com/pdcgo/shared/db_models"
	"github.com/wargasipil/data_processing/types"
)

type WithdrawalD struct {
	Withdrawal types.Series[*db_models.InvoItem]
	List       types.Series[EarningList]
}

type WithdrawalDataFrame struct {
	count  int
	offset []int
	D      *WithdrawalD
}

func NewWithdrawalDataFrame(datas []*Withdrawal) *WithdrawalDataFrame {
	d := &WithdrawalDataFrame{
		count:  0,
		offset: []int{},
		D: &WithdrawalD{
			Withdrawal: types.Series[*db_models.InvoItem]{},
			List:       types.Series[EarningList]{},
		},
	}

	for _, item := range datas {
		d.D.Withdrawal = append(d.D.Withdrawal, item.Withdrawal)
		d.D.List = append(d.D.List, item.List)
		d.offset = append(d.offset, d.count)
		d.count++
	}

	return d
}

func (d *WithdrawalDataFrame) Query(filters ...types.OffsetFilter) *WithdrawalDataFrame {
	offset := d.offset
	for _, filter := range filters {
		offset = filter(offset)
	}

	newdf := WithdrawalDataFrame{
		count:  len(offset),
		D:      d.D,
		offset: offset,
	}

	return &newdf
}

func (d *WithdrawalDataFrame) get(i int) *Withdrawal {
	item := Withdrawal{
		Withdrawal: d.D.Withdrawal[i],
		List:       d.D.List[i],
	}

	return &item
}

func (d *WithdrawalDataFrame) First() *Withdrawal {
	if len(d.offset) == 0 {
		return nil
	}

	i := d.offset[0]
	return d.get(i)
}

func (d *WithdrawalDataFrame) Last() *Withdrawal {
	if len(d.offset) == 0 {
		return nil
	}

	i := d.offset[len(d.offset)-1]
	return d.get(i)
}

func (d *WithdrawalDataFrame) Data() []*Withdrawal {
	result := []*Withdrawal{}
	for _, i := range d.offset {
		result = append(result, d.get(i))
	}

	return result
}
