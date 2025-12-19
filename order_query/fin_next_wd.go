package order_query

import (
	"time"

	"github.com/pdcgo/shared/db_models"
	"gorm.io/gorm"
)

type nextWdImpl struct {
	tx         *gorm.DB
	mpID       uint
	after      time.Time
	diffAmount float64
}

// Get implements finance_iface.NextWithdrawalQuery.
func (n *nextWdImpl) Get(wd *db_models.Withdrawal) error {
	return n.buildQuery().Find(wd).Error
}

// IncActualAmount implements finance_iface.NextWithdrawalQuery.
func (n *nextWdImpl) IncActualAmount(amount float64) error {
	var idnya uint
	err := n.buildQuery().Select("id").Find(&idnya).Error
	if err != nil {
		return err
	}

	qitem := wdByID{
		tx:   n.tx,
		wdID: idnya,
	}

	return qitem.IncActualAmount(amount)
}

func (n *nextWdImpl) buildQuery() *gorm.DB {
	return n.tx.
		Model(&db_models.Withdrawal{}).
		Where("at >= ?", n.after).
		Where("diff_amount = ?", n.diffAmount).
		Where("mp_id = ?", n.mpID).
		Order("at asc")
}
