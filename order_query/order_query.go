package order_query

import (
	"errors"
	"fmt"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"github.com/pdcgo/shared/pkg/streampipe"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type OrderDataQuery interface {
	Lock() error
	LogAdjustment(tipe db_models.AdjustmentType, at time.Time, fundAt time.Time, amount float64, desc string) (uint, error)
	// Completed(at time.Time) error
	// ReturnClaim(amount float64) error
}

type HaveMarketplaceRes struct {
	ValidCount   int   `json:"valid_count"`
	InvalidCount int   `json:"invalid_count"`
	Err          error `gorm:"-"`
}

func (mpres *HaveMarketplaceRes) GetError() error {
	if mpres.Err != nil {
		return mpres.Err
	}

	if mpres.ValidCount == 0 {
		return fmt.Errorf("marketplace yang dipilih kemungkinan salah karena valid count order 0")
	}

	if mpres.InvalidCount != 0 {
		return fmt.Errorf("ada %d order tidak valid karena memiliki marketplace yang berbeda", mpres.InvalidCount)
	}

	return nil
}

type OrderRefIDsQuery interface {
	Lock() error
	Completed(at time.Time) error
	// GetIDs() ([]uint, error)
	HaveMarketplace(mpID uint) *HaveMarketplaceRes
	ChangeMarketplace(mpID uint) error
}

type OrderQuery interface {
	// ByID(teamID uint, orderID uint) OrderDataQuery
	// ByIDs(orderIDs []uint) BulkIDs

	ByRefID(teamID uint, refID string) OrderDataQuery
	ByRefIDs(teamID uint, refIDs []string) OrderRefIDsQuery

	// OrderItemByQuery(q func(qdb *gorm.DB) *gorm.DB) OrderItemByQuery
}

func NewOrderQuery(tx *gorm.DB, agent identity_iface.Agent, pub streampipe.PublishProvider) OrderQuery {
	return &orderQueryImpl{
		tx:    tx,
		agent: agent,
		pub:   pub,
	}
}

type orderQueryImpl struct {
	tx    *gorm.DB
	agent identity_iface.Agent
	pub   streampipe.PublishProvider
}

// ByRefIDs implements OrderQuery.
func (o *orderQueryImpl) ByRefIDs(teamID uint, refIDs []string) OrderRefIDsQuery {
	return &orderByRefIDsImpl{
		teamID: teamID,
		refIDs: refIDs,
		tx:     o.tx,
		agent:  o.agent,
		pub:    o.pub,
	}
}

// ByRefID implements OrderQuery.
func (o *orderQueryImpl) ByRefID(teamID uint, refID string) OrderDataQuery {
	return &orderByRefIDImpl{
		teamID: teamID,
		refID:  refID,
		tx:     o.tx,
		agent:  o.agent,
		pub:    o.pub,
	}
}

type MpItemQuery interface {
	Lock() error
	// CheckBankAccount() error
	// CheckHoldAsset() error
	CreateWithdrawal(wdTime time.Time, amount float64, afterAmount float64) (*db_models.Withdrawal, error)
	// DeleteWithdrawal(wdID uint) error
}

type MarketplaceQuery interface {
	ByID(teamID, mpID uint) MpItemQuery
}

type CreateAssetPayload struct {
	TeamID    uint
	AssetType db_models.AssetType
	Name      string
}

type FinanceMpWithdrawal interface {
	// CreateManualWithdrawal(wdTime time.Time, fromAssetID, toAssetID uint, amount float64) (*db_models.AssetHistory, error)
	SetWithdrawal(wdTime time.Time, teamID, mpID, fromAssetID, toAssetID uint, amount float64, afterAmount float64) (*db_models.Withdrawal, error)
	CreateAsset(payload *CreateAssetPayload) (*db_models.Asset, error)
}

type WithdrawalIDQuery interface {
	IncActualAmount(amount float64) error
}

type NextWithdrawalQuery interface {
	WithdrawalIDQuery
	Get(wd *db_models.Withdrawal) error
}

type FinDataQuery interface {
	WithdrawalByID(wdID uint) WithdrawalIDQuery
	NextWithdrawal(mpID uint, after time.Time, diffAmount float64) NextWithdrawalQuery
}

type Finance interface {
	MpWithdrawal(agent identity_iface.Agent, db *gorm.DB) FinanceMpWithdrawal
	DataQuery(agent identity_iface.Agent, tx *gorm.DB) FinDataQuery
}

func NewMarketplaceQuery(fin Finance, tx *gorm.DB, agent identity_iface.Agent) MarketplaceQuery {
	return &marketplaceQueryImpl{
		fin:   fin,
		tx:    tx,
		agent: agent,
	}
}

type marketplaceQueryImpl struct {
	fin   Finance
	tx    *gorm.DB
	agent identity_iface.Agent
}

// ByID implements order_iface.MarketplaceQuery.
func (m *marketplaceQueryImpl) ByID(teamID uint, mpID uint) MpItemQuery {
	wdman := m.fin.MpWithdrawal(m.agent, m.tx)
	return &mpByIDImpl{
		wdman:  wdman,
		teamID: teamID,
		mpID:   mpID,
		tx:     m.tx,
		agent:  m.agent,
	}
}

type mpByIDImpl struct {
	wdman  FinanceMpWithdrawal
	teamID uint
	mpID   uint
	tx     *gorm.DB
	agent  identity_iface.Agent
}

func (m *mpByIDImpl) buildQuery(lock bool) *gorm.DB {
	tx := m.tx
	if lock {
		tx = tx.Clauses(clause.Locking{
			Strength: "UPDATE",
			Options:  "NOWAIT",
		})
	}
	return tx.
		Model(&db_models.Marketplace{}).
		Where("team_id = ?", m.teamID).
		Where("id = ?", m.mpID)
}

// CheckBankAccount implements order_iface.MpItemQuery.
func (m *mpByIDImpl) CheckBankAccount() error {
	var err error

	var marketplace db_models.Marketplace
	err = m.buildQuery(false).Find(&marketplace).Error
	if err != nil {
		return err
	}

	bankAccID := uint(0)
	if marketplace.BankAccountID != nil {
		bankAccID = *marketplace.BankAccountID
	}
	if bankAccID != 0 {
		return err
	}

	asset, err := m.wdman.CreateAsset(&CreateAssetPayload{
		TeamID:    marketplace.TeamID,
		AssetType: db_models.ABankAccount,
		Name:      marketplace.MpName,
	})
	if err != nil {
		return err
	}

	bank := db_models.BankAccount{
		AssetID: asset.ID,
	}
	err = m.tx.Save(&bank).Error
	if err != nil {
		return err
	}

	err = m.tx.
		Model(&db_models.Marketplace{}).
		Where("id = ?", marketplace.ID).
		Updates(map[string]interface{}{
			"bank_account_id": bank.ID,
		}).
		Error
	if err != nil {
		return err
	}

	return nil
}

// CheckHoldAsset implements order_iface.MpItemQuery.
func (m *mpByIDImpl) CheckHoldAsset() error {
	var err error

	var marketplace db_models.Marketplace
	err = m.buildQuery(false).Find(&marketplace).Error
	if err != nil {
		return err
	}

	if marketplace.ID == 0 {
		return gorm.ErrRecordNotFound
	}

	var asset *db_models.Asset
	holdAssetID := uint(0)
	if marketplace.HoldAssetID != nil {
		holdAssetID = *marketplace.HoldAssetID
	}
	if holdAssetID != 0 { // creating hold asset id jika 0
		return nil
	}

	asset, err = m.wdman.CreateAsset(&CreateAssetPayload{
		TeamID:    marketplace.TeamID,
		AssetType: db_models.MpHoldingFund,
		Name:      marketplace.MpName,
	})
	if err != nil {
		return err
	}
	err = m.tx.
		Model(&db_models.Marketplace{}).
		Where("id = ?", marketplace.ID).
		Updates(map[string]interface{}{
			"hold_asset_id": asset.ID,
		}).
		Error
	if err != nil {
		return err
	}

	return nil
}

// CreateWithdrawal implements order_iface.MpItemQuery.
func (m *mpByIDImpl) CreateWithdrawal(wdTime time.Time, amount float64, afterAmount float64) (*db_models.Withdrawal, error) {
	var market db_models.Marketplace
	err := m.buildQuery(false).Preload("BankAccount").Find(&market).Error
	if err != nil {
		return nil, err
	}

	return m.wdman.SetWithdrawal(wdTime, m.teamID, m.mpID, *market.HoldAssetID, market.BankAccount.AssetID, amount, afterAmount)
}

// DeleteWithdrawal implements order_iface.MpItemQuery.
func (m *mpByIDImpl) DeleteWithdrawal(wdID uint) error {
	var wd db_models.Withdrawal
	err := m.tx.Model(&db_models.Withdrawal{}).First(&wd, wdID).Error
	if err != nil {
		return err
	}
	histID := wd.HistID
	err = m.tx.Model(&db_models.WdOrderNotFound{}).Where("wd_id = ?", wd.ID).Delete(&db_models.WdOrderNotFound{}).Error
	if err != nil {
		return err
	}

	err = m.tx.Model(&db_models.Withdrawal{}).Delete(&wd).Error
	if err != nil {
		return err
	}

	err = m.tx.Model(&db_models.AssetHistory{}).Where("id = ?", histID).Error
	if err != nil {
		return err
	}

	return nil
}

// Lock implements order_iface.MpItemQuery.
func (m *mpByIDImpl) Lock() error {
	var id uint
	err := m.buildQuery(true).Select("id").Find(&id).Error
	if err != nil {
		return err
	}

	if id == 0 {
		return errors.New("cannot lock marketplace not found")
	}
	return nil
}
