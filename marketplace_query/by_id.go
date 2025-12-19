package marketplace_query

import (
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type byIDImpl struct {
	tx     *gorm.DB
	agent  identity_iface.Agent
	teamID uint
	mpID   uint
}

func (b *byIDImpl) buildQuery(lock bool) *gorm.DB {
	tx := b.tx
	if lock {
		tx = tx.Clauses(clause.Locking{
			Strength: "UPDATE",
			Options:  "NOWAIT",
		})
	}
	query := tx.
		Model(&db_models.Marketplace{}).
		Where("team_id = ?", b.teamID).
		Where("id = ?", b.mpID)

	return query
}
