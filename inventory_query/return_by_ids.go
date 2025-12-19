package inventory_query

import (
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func NewReturnByIds(
	tx *gorm.DB,
	agent identity_iface.Agent,

	teamID uint,
	txIDs []uint,
) ReturnQuery {
	return &returnByIdsImpl{
		tx:     tx,
		agent:  agent,
		teamID: teamID,
		txIDs:  txIDs,
	}
}

type returnByIdsImpl struct {
	tx    *gorm.DB
	agent identity_iface.Agent

	teamID uint
	txIDs  []uint
}

// AddNote implements inventory_iface.ReturnQuery.
func (r *returnByIdsImpl) AddNote(orderID uint, tipe db_models.NoteType, note string) error {
	panic("unimplemented")
}

// Err implements inventory_iface.ReturnQuery.
func (r *returnByIdsImpl) Err() error {
	panic("unimplemented")
}

// Get implements inventory_iface.ReturnQuery.
func (r *returnByIdsImpl) Get(lock bool) error {
	panic("unimplemented")
}

// SetResolution implements inventory_iface.ReturnQuery.
func (r *returnByIdsImpl) SetResolution(resID uint) ReturnQuery {
	panic("unimplemented")
}

// SetReturnProblem implements inventory_iface.ReturnQuery.
func (r *returnByIdsImpl) SetReturnProblem() ReturnQuery {
	panic("unimplemented")
}

// Cancel implements invertory_iface.ReturnQuery.
func (r *returnByIdsImpl) Cancel() error {
	txIDs := []uint{}

	err := r.buildQuery(true).Select("id").Find(&txIDs).Error
	if err != nil {
		return err
	}

	query := r.buildQuery(false)
	err = query.Update("status", db_models.InvTxCancel).Error
	if err != nil {
		return err
	}

	return r.logCancel(txIDs)

}

func (r *returnByIdsImpl) logCancel(txIDs []uint) error {
	var err error
	for _, txID := range txIDs {
		invTs := &db_models.InvTimestamp{
			TxID:      txID,
			UserID:    r.agent.GetUserID(),
			Status:    db_models.InvTxCancel,
			Timestamp: time.Now(),
			From:      r.agent.GetAgentType(),
		}

		err = r.tx.Save(&invTs).Error
		if err != nil {
			return err
		}
	}

	return err

}

func (r *returnByIdsImpl) buildQuery(lock bool) *gorm.DB {
	tx := r.tx
	if lock {
		tx = tx.Clauses(clause.Locking{
			Strength: "UPDATE",
			Options:  "NOWAIT",
		})
	}
	query := tx.
		Model(&db_models.InvTransaction{}).
		Where("team_id = ?", r.teamID).
		Where("type IN ?", []db_models.InvTxType{
			db_models.InvTxRestock,
			db_models.InvTxReturn,
		}).
		Where("id IN ?", r.txIDs)

	return query
}
