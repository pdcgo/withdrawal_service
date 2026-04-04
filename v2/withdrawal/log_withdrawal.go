package withdrawal

import (
	"time"

	"gorm.io/gorm"
)

type V2WithdrawalLog struct {
	ID     uint `gorm:"primarykey"`
	TeamId uint
	ShopId uint

	Amount float64

	UserId    uint
	At        time.Time
	CreatedAt time.Time
}

func (w *wdServiceImpl) logWithdrawal(db *gorm.DB, wlog *V2WithdrawalLog) error {
	err := db.Create(wlog).Error
	return err
}
