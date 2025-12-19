package order_query

import (
	"fmt"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"github.com/pdcgo/shared/pkg/streampipe"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type orderByRefIDImpl struct {
	teamID uint
	refID  string
	tx     *gorm.DB
	agent  identity_iface.Agent
	pub    streampipe.PublishProvider
}

// Lock implements OrderDataQuery.
func (o *orderByRefIDImpl) Lock() error {
	ord := &db_models.Order{}
	err := o.buildQuery(true).
		Where("status != ?", db_models.OrdCancel).
		Select("id").Find(&ord).Error

	if err != nil {
		return err
	}

	if ord.ID == 0 {
		return gorm.ErrRecordNotFound
	}

	return nil
}

func (o *orderByRefIDImpl) buildQuery(lock bool) *gorm.DB {

	tx := o.tx
	if lock {
		tx = tx.Clauses(clause.Locking{
			Strength: "UPDATE",
			Options:  "NOWAIT",
		})
	}
	query := tx.
		Model(&db_models.Order{}).
		Where("team_id = ?", o.teamID).
		Where("order_ref_id = ?", o.refID)
		// Where("status != ?", db_models.OrdCancel)
	return query
}

// LogAdjustment implements OrderDataQuery.
func (o *orderByRefIDImpl) LogAdjustment(tipe db_models.AdjustmentType, at time.Time, fundAt time.Time, amount float64, desc string) (uint, error) {
	ord := db_models.Order{}
	err := o.buildQuery(false).
		Where("status != ?", db_models.OrdCancel).
		Find(&ord).Error
	if err != nil {
		return 0, err
	}

	if ord.OrderMpID == 0 {
		return 0, fmt.Errorf("refmode order with id %s marketplace not set %d with receipt %s with ref %s", ord.OrderRefID, ord.ID, ord.Receipt, o.refID)
	}

	adj, err := o.getAdjustment(&ord, tipe)
	if err != nil {
		return 0, err
	}

	// var logtipe db_models.OrderAdjLogType

	if adj.ID == 0 {
		// logtipe = db_models.AdjLogCreated
		adj = &db_models.OrderAdjustment{
			OrderID: ord.ID,
			MpID:    ord.OrderMpID,
			At:      at,
			FundAt:  fundAt,
			Type:    tipe,
			Amount:  amount,
			Desc:    desc,
		}

		err = o.tx.Save(adj).Error
		if err != nil {
			return 0, err
		}

	} else {
		// logtipe = db_models.AdjLogUpdated
		adj.Amount = amount
		adj.At = at
		adj.FundAt = at
		err = o.tx.Save(adj).Error

		if err != nil {
			return 0, err
		}
	}

	// log := db_models.OrderAdjustmentLog{
	// 	AdjID:     adj.ID,
	// 	UserID:    o.agent.GetUserID(),
	// 	OrderID:   adj.OrderID,
	// 	From:      o.agent.GetAgentType(),
	// 	LogType:   logtipe,
	// 	Data:      datatypes.NewJSONType[*db_models.OrderAdjustment](adj),
	// 	Timestamp: time.Now(),
	// }

	// err = o.tx.Save(&log).Error
	// if err != nil {
	// 	return adj.ID, err
	// }

	err = o.buildQuery(false).
		Updates(map[string]interface{}{
			"wd_total": o.tx.
				Model(&db_models.OrderAdjustment{}).
				Select("SUM(CASE WHEN amount > 0 THEN amount ELSE 0 END)").
				Where("order_adjustments.order_id = orders.id"),
			"adjustment": o.tx.
				Model(&db_models.OrderAdjustment{}).
				Select("SUM(CASE WHEN amount < 0 THEN amount ELSE 0 END)").
				Where("order_adjustments.order_id = orders.id"),
		}).
		Error

	return adj.ID, err
}

func (o *orderByRefIDImpl) getAdjustment(ord *db_models.Order, tipe db_models.AdjustmentType) (*db_models.OrderAdjustment, error) {
	ordAdjust := db_models.OrderAdjustment{}
	err := o.tx.
		Model(&db_models.OrderAdjustment{}).
		Where("order_id = ?", ord.ID).
		Where("type = ?", tipe).
		Find(&ordAdjust).
		Error

	return &ordAdjust, err
}
