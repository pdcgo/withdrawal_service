package marketplace_query

import (
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"github.com/pdcgo/withdrawal_service/order_query"
	"gorm.io/gorm"
)

type ItemQuery interface {
	FirstOrder() (ftime time.Time, err error)
	Get() (*db_models.Marketplace, error)
	Lock() error
	CheckBankAccount() error
	CheckHoldAsset() error
	CreateWithdrawal(wdTime time.Time, amount float64, afterAmount float64) (*db_models.Withdrawal, error)
	DeleteWithdrawal(wdID uint) error
}

type MarketplaceQuery interface {
	ByUsername(teamID uint, mptype db_models.MarketplaceType, username string) ItemQuery
	ByID(teamID, mpID uint) ItemQuery
}

func NewMarketplaceQuery(tx *gorm.DB, agent identity_iface.Agent) MarketplaceQuery {
	wdman := order_query.NewMpWithdrawal(agent, tx)
	return &marketplaceQueryImpl{
		tx:    tx,
		agent: agent,
		wdman: wdman,
	}
}

type marketplaceQueryImpl struct {
	wdman order_query.FinanceMpWithdrawal
	tx    *gorm.DB
	agent identity_iface.Agent
}

// ByID implements marketplace_iface.MarketplaceQuery.
func (m *marketplaceQueryImpl) ByID(teamID uint, mpID uint) ItemQuery {
	return &itemQueryImpl{
		wdman: m.wdman,
		tx:    m.tx,
		query: &byIDImpl{
			tx:     m.tx,
			agent:  m.agent,
			teamID: teamID,
			mpID:   mpID,
		},
	}
}

// ByUsername implements marketplace_iface.MarketplaceQuery.
func (m *marketplaceQueryImpl) ByUsername(teamID uint, mptype db_models.MarketplaceType, username string) ItemQuery {
	return &itemQueryImpl{
		wdman: m.wdman,
		tx:    m.tx,
		query: &byUsernameImpl{
			tx:       m.tx,
			agent:    m.agent,
			username: username,
			teamID:   teamID,
			mptype:   mptype,
		},
	}
}
