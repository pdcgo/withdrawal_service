package order_query

import (
	"time"

	"github.com/pdcgo/shared/interfaces/identity_iface"
	"gorm.io/gorm"
)

func NewFinDataQuery(agent identity_iface.Agent, tx *gorm.DB) FinDataQuery {
	return &finDataQueryImpl{
		agent: agent,
		tx:    tx,
	}
}

type finDataQueryImpl struct {
	fin   Finance
	agent identity_iface.Agent
	tx    *gorm.DB
}

// NextWithdrawal implements FinDataQuery.
func (f *finDataQueryImpl) NextWithdrawal(mpID uint, after time.Time, diffAmount float64) NextWithdrawalQuery {
	return &nextWdImpl{
		tx:         f.tx,
		mpID:       mpID,
		after:      after,
		diffAmount: diffAmount,
	}
}

// WithdrawalByID implements FinDataQuery.
func (f *finDataQueryImpl) WithdrawalByID(wdID uint) WithdrawalIDQuery {
	return &wdByID{
		tx:   f.tx,
		wdID: wdID,
	}
}
