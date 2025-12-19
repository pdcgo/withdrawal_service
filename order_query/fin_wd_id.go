package order_query

import (
	"github.com/pdcgo/shared/db_models"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type wdByID struct {
	tx   *gorm.DB
	wdID uint
}

// IncActualAmount implements finance_iface.WithdrawalIDQuery.
func (w *wdByID) IncActualAmount(amount float64) error {
	err := w.buildQuery(false).
		Update("diff_amount", gorm.Expr("diff_amount + ?", amount)).Error
	return err
}

func (w *wdByID) buildQuery(lock bool) *gorm.DB {
	tx := w.tx
	if lock {
		tx = tx.Clauses(clause.Locking{
			Strength: "UPDATE",
			Options:  "NOWAIT",
		})
	}

	return tx.Model(&db_models.Withdrawal{}).Where("id = ?", w.wdID)
}
