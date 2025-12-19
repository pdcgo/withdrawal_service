package order_query

import (
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"gorm.io/gorm"
)

type MpWithdrawal struct {
	agent identity_iface.Agent
	tx    *gorm.DB
}

// CreateAsset implements FinanceMpWithdrawal.
func (m *MpWithdrawal) CreateAsset(payload *CreateAssetPayload) (*db_models.Asset, error) {
	asset := db_models.Asset{
		TeamID:    payload.TeamID,
		AssetType: payload.AssetType,
		Name:      payload.Name,
	}
	err := m.tx.Save(&asset).Error
	return &asset, err
}

// SetWithdrawal implements FinanceMpWithdrawal.
func (m *MpWithdrawal) SetWithdrawal(wdTime time.Time, teamID uint, mpID uint, fromAssetID uint, toAssetID uint, amount float64, afterAmount float64) (*db_models.Withdrawal, error) {
	var err error
	// checking withdrawal
	var withdrawal db_models.Withdrawal
	err = m.tx.
		Model(&db_models.Withdrawal{}).
		Joins("JOIN asset_histories ON asset_histories.id = withdrawals.hist_id").
		Where("asset_histories.from_asset_id = ?", fromAssetID).
		Where("mp_id = ?", mpID).
		Where("asset_histories.amount = ?", amount).
		Where("asset_histories.at = ?", wdTime).
		Find(&withdrawal).
		Error
	if err != nil {
		return nil, err
	}

	if withdrawal.ID != 0 {

		if withdrawal.DiffAmount == 0 {
			return &withdrawal, nil
		}

		err = m.tx.Model(&db_models.Withdrawal{}).
			Select("diff_amount").
			Where("id = ?", withdrawal.ID).
			Updates(map[string]interface{}{
				"diff_amount": m.tx.Model(&db_models.AssetHistory{}).
					Select("amount * -1").
					Where("id = ?", withdrawal.HistID),
			}).Error
		if err != nil {
			return nil, err
		}
		return &withdrawal, nil
	}

	// creating history
	hist, err := m.createAssetHistory(wdTime, fromAssetID, toAssetID, amount)
	if err != nil {
		return nil, err
	}

	withdrawal = db_models.Withdrawal{
		CreatedByID: m.agent.GetUserID(),
		HistID:      hist.ID,
		TeamID:      teamID,
		MpID:        mpID,
		DiffAmount:  amount * -1,
		AfterAmount: afterAmount,
		At:          wdTime,
		IsNew:       true,
	}

	err = m.tx.Save(&withdrawal).Error
	if err != nil {
		return nil, err
	}

	return &withdrawal, nil
}

// SetWithdrawal implements order_service.FinanceMpWithdrawal.
func (m *MpWithdrawal) createAssetHistory(wdTime time.Time, fromAssetID, toAssetID uint, amount float64) (*db_models.AssetHistory, error) {
	// log.Println("create tx", wdTime.String())
	hist := db_models.AssetHistory{
		CreatedByID: m.agent.GetUserID(),
		FromAssetID: fromAssetID,
		ToAssetID:   toAssetID,
		Type:        db_models.AssetFund,
		At:          wdTime,
		Amount:      amount,
		From:        m.agent.GetAgentType(),
	}
	err := m.tx.Create(&hist).Error

	return &hist, err
}

func NewMpWithdrawal(agent identity_iface.Agent, tx *gorm.DB) *MpWithdrawal {
	return &MpWithdrawal{
		tx:    tx,
		agent: agent,
	}
}
