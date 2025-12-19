package marketplace_query

import (
	"errors"
	"fmt"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/withdrawal_service/order_query"
	"gorm.io/gorm"
)

type itemQuery interface {
	buildQuery(lock bool) *gorm.DB
}

type itemQueryImpl struct {
	wdman order_query.FinanceMpWithdrawal
	query itemQuery
	tx    *gorm.DB
}

// type timeRes struct {
// 	OrderTime time.Time
// }

func (m *itemQueryImpl) parseTime(data string, layouts ...string) (time.Time, error) {
	var errd error
	for _, layout := range layouts {
		tt, err := time.Parse(layout, data)
		if err == nil {
			return tt, nil
		} else {
			errd = err
		}
	}

	return time.Time{}, errd
}

// FirstOrder implements marketplace_iface.ItemQuery.
func (m *itemQueryImpl) FirstOrder() (ftime time.Time, err error) {
	var marketplace *db_models.Marketplace
	marketplace, err = m.Get()
	if err != nil {
		return time.Time{}, err
	}

	var first string
	var tt time.Time

	err = m.tx.Raw(`
	SELECT 
		MIN(orders.order_time) as order_time
	FROM orders
	WHERE order_mp_id = ?
	`, marketplace.ID).
		Scan(&first).
		Error

	if err != nil {
		return tt, fmt.Errorf("Toko %s belum punya order", marketplace.MpName)
	}

	if first == "" {
		return tt, fmt.Errorf("%s dont have order", marketplace.MpUsername)
	}

	tt, err = m.parseTime(
		first,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05.999999999-07:00",
	)
	if err != nil {
		return tt, err
	}

	if tt.IsZero() {
		return tt, errors.New("ada order time belum di set, atau order di marketplace kosong")
	}

	return tt, nil
}

// DeleteWithdrawal implements order_iface.MpItemQuery.
func (m *itemQueryImpl) DeleteWithdrawal(wdID uint) error {
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

// CreateWithdrawal implements order_iface.MpItemQuery.
func (m *itemQueryImpl) CreateWithdrawal(wdTime time.Time, amount float64, afterAmount float64) (*db_models.Withdrawal, error) {
	var market db_models.Marketplace
	err := m.query.buildQuery(false).Preload("BankAccount").Find(&market).Error
	if err != nil {
		return nil, err
	}

	return m.wdman.SetWithdrawal(wdTime, market.TeamID, market.ID, *market.HoldAssetID, market.BankAccount.AssetID, amount, afterAmount)
}

// Lock implements order_iface.MpItemQuery.
func (m *itemQueryImpl) Lock() error {
	var id uint
	err := m.query.buildQuery(true).Select("id").Find(&id).Error
	if err != nil {
		return err
	}

	if id == 0 {
		return errors.New("cannot lock marketplace not found")
	}
	return nil
}

// CheckBankAccount implements order_iface.MpItemQuery.
func (m *itemQueryImpl) CheckBankAccount() error {
	var err error

	var marketplace db_models.Marketplace
	err = m.query.buildQuery(false).Find(&marketplace).Error
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

	asset, err := m.wdman.CreateAsset(&order_query.CreateAssetPayload{
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
func (m *itemQueryImpl) CheckHoldAsset() error {
	var err error

	var marketplace db_models.Marketplace
	err = m.query.buildQuery(false).Find(&marketplace).Error
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

	asset, err = m.wdman.CreateAsset(&order_query.CreateAssetPayload{
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

// Get implements marketplace_iface.ItemQuery.
func (m *itemQueryImpl) Get() (*db_models.Marketplace, error) {
	hasil := db_models.Marketplace{}
	err := m.query.buildQuery(false).Find(&hasil).Error
	if err != nil {
		return &hasil, err
	}
	if hasil.ID == 0 {
		return &hasil, ErrMarketplaceNotFound
	}
	return &hasil, err
}
