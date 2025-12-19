package order_query

import (
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"gorm.io/gorm"
)

func NewFinance(agent identity_iface.Agent, tx *gorm.DB) Finance {
	return &financeImpl{
		agent: agent,
		tx:    tx,
	}
}

type financeImpl struct {
	agent identity_iface.Agent
	tx    *gorm.DB
}

// DataQuery implements Finance.
func (f *financeImpl) DataQuery(agent identity_iface.Agent, tx *gorm.DB) FinDataQuery {
	return NewFinDataQuery(f.agent, f.tx)
}

// MpWithdrawal implements Finance.
func (f *financeImpl) MpWithdrawal(agent identity_iface.Agent, db *gorm.DB) FinanceMpWithdrawal {
	return NewMpWithdrawal(agent, db)
}
