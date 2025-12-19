package marketplace_query

import (
	"errors"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var ErrMarketplaceNotFound = errors.New("marketplace not found")

type byUsernameImpl struct {
	tx       *gorm.DB
	agent    identity_iface.Agent
	username string
	teamID   uint
	mptype   db_models.MarketplaceType
}

func (b *byUsernameImpl) buildQuery(lock bool) *gorm.DB {
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
		Where("mp_type = ?", b.mptype).
		Where("mp_username = ?", b.username).
		Where("(deleted IS NULL OR deleted = ?)", false)

	return query
}
