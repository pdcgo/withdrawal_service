package order_query

import (
	"errors"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/events/order_event"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"github.com/pdcgo/shared/pkg/streampipe"
	"github.com/pdcgo/withdrawal_service/inventory_query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type orderByRefIDsImpl struct {
	teamID uint
	refIDs []string
	tx     *gorm.DB
	agent  identity_iface.Agent
	pub    streampipe.PublishProvider
}

// ChangeMarketplace implements OrderRefIDsQuery.
func (o *orderByRefIDsImpl) ChangeMarketplace(mpID uint) error {
	return o.
		buildQuery(false).
		Where("order_mp_id != ?", mpID).
		Update("order_mp_id", mpID).Error // perlu unittest
}

// HaveMarketplace implements OrderRefIDsQuery.
func (o *orderByRefIDsImpl) HaveMarketplace(mpID uint) *HaveMarketplaceRes {
	hasil := HaveMarketplaceRes{
		ValidCount:   0,
		InvalidCount: 0,
	}
	dbquery := o.buildQuery(false)
	dbquery = dbquery.
		Select(
			`
				SUM(CASE WHEN orders.order_mp_id = ? THEN 1 ELSE 0 END) as valid_count,
				SUM(CASE WHEN orders.order_mp_id != ? THEN 1 ELSE 0 END) as invalid_count
			`, mpID, mpID,
		)

	hasil.Err = dbquery.
		Find(&hasil).
		Error

	if hasil.Err == nil {
		if hasil.ValidCount == 0 && hasil.InvalidCount == 0 {
			hasil.Err = errors.New("does not have any order in refids query")
		}
	}
	return &hasil
}

// Completed implements OrderRefIDsQuery.
func (o *orderByRefIDsImpl) Completed(at time.Time) error {
	err := o.buildQuery(false).
		Updates(map[string]interface{}{
			"wd_fund":    true,
			"wd_fund_at": at,
		}).Error
	if err != nil {
		return err
	}
	completeIDs, err := o.completedIDs()
	if err != nil {
		return err
	}

	err = o.buildQuery(false).
		Where("wd_total > 0").
		Where("status NOT IN ?", []db_models.OrdStatus{
			db_models.OrdReturnCompleted,
			db_models.OrdProblem,
			db_models.OrdCancel,
			db_models.OrdCompleted,
		}).
		Update("status", db_models.OrdCompleted).
		Error

	if err != nil {
		return err
	}

	err = o.setLogCompleted(completeIDs)
	if err != nil {
		return err
	}

	// canceling return tx id
	err = o.setReturnToCancel(completeIDs)
	if err != nil {
		return err
	}

	for _, com := range completeIDs {
		o.pub.Send(streampipe.DEFAULT_TOPIC, &order_event.OrderEvent{
			Action:  order_event.OrderChangeStatus,
			Status:  db_models.OrdCompleted,
			OrderID: com,
		})
	}

	return nil
}

// Lock implements OrderRefIDsQuery.
func (o *orderByRefIDsImpl) Lock() error {
	return o.buildQuery(true).Select("id").Find(&[]*db_models.Order{}).Error
}

func (o *orderByRefIDsImpl) buildQuery(lock bool) *gorm.DB {

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
		Where("order_ref_id IN ?", o.refIDs)
	return query
}

func (o *orderByRefIDsImpl) completedIDs() ([]uint, error) {
	hasil := []uint{}
	err := o.buildQuery(false).
		Select("id").
		Where("wd_total > 0").
		Where("status NOT IN ?", []db_models.OrdStatus{
			db_models.OrdReturnCompleted,
			db_models.OrdProblem,
			db_models.OrdCancel,
			db_models.OrdCompleted,
		}).
		Find(&hasil).Error
	return hasil, err
}

func (o *orderByRefIDsImpl) setLogCompleted(completeIDs []uint) error {
	var err error

	for _, id := range completeIDs {
		ts := db_models.OrderTimestamp{
			OrderID:     id,
			UserID:      o.agent.GetUserID(),
			OrderStatus: db_models.OrdCompleted,
			Timestamp:   time.Now(),
			From:        o.agent.GetAgentType(),
		}
		err = o.tx.Save(&ts).Error

		if err != nil {
			return err
		}
	}
	return nil
}

func (o *orderByRefIDsImpl) setReturnToCancel(ordIDs []uint) error {
	txIDs := []uint{}
	err := o.tx.
		Model(&db_models.Order{}).
		Select("DISTINCT invertory_return_tx_id").
		Where("invertory_return_tx_id IS NOT NULL").
		Where("id IN ?", ordIDs).
		Find(&txIDs).
		Error
	if err != nil {
		return err
	}

	retQuery := inventory_query.NewDataQuery(o.tx, o.agent).ReturnByIDs(o.teamID, txIDs)
	return retQuery.Cancel()
}
