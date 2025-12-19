package withdrawal_service

import (
	"github.com/pdcgo/shared/db_models"
	"gorm.io/gorm"
)

type orderRepoImpl struct {
	db *gorm.DB
}

// OrderByExternalID implements OrderRepo.
func (o *orderRepoImpl) OrderByExternalID(extID string) (*db_models.Order, error) {
	ord := db_models.Order{}
	err := o.db.
		Model(&db_models.Order{}).
		Where("order_ref_id = ?", extID).
		Where("status != ?", db_models.OrdCancel).
		Find(&ord).
		Error

	if err != nil {
		return &ord, err
	}

	return &ord, nil
}

func NewOrderRepo(db *gorm.DB) *orderRepoImpl {
	return &orderRepoImpl{
		db: db,
	}
}
