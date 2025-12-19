package withdrawal_service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"github.com/pdcgo/shared/pkg/streampipe"
	"github.com/pdcgo/withdrawal_service/marketplace_query"
	"github.com/pdcgo/withdrawal_service/order_query"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

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

type ImporterProcessor interface {
	SetFilterMarketplace(mp marketplace_query.ItemQuery) error
	OrderFund(item *db_models.InvoItem) error
	OrderAdjustment(item *db_models.InvoItem) error
	Unknown(item *db_models.InvoItem) error
	Withdrawal(item *db_models.InvoItem) error
	Check(sisaAmount float64) error
}

func NewImporterProcessor(
	db *gorm.DB,
	fin order_query.Finance,
	ctx context.Context,
	query *WDImporterQuery,
	agent identity_iface.Agent,
	pub streampipe.PublishProvider,

) ImporterProcessor {
	ctxValue := context.WithValue(ctx, "controller", "importer_processor")
	return &importerProcessorImpl{
		db:        db.WithContext(ctxValue),
		fin:       fin,
		query:     query,
		agent:     agent,
		itemlists: InvoList{},
		pub:       pub,
	}
}

type InvoList []*db_models.InvoItem

func (m InvoList) Amount() float64 {
	var hasil float64
	for _, val := range m {
		hasil += val.Amount
	}

	return hasil
}

func (m InvoList) OrderRefIDs() []string {
	hasil := []string{}
	maper := map[string]bool{}
	for _, item := range m {
		maper[item.ExternalOrderID] = true
	}

	for id := range maper {
		hasil = append(hasil, id)
	}

	return hasil
}

func (m InvoList) First() *db_models.InvoItem {
	return m[0]
}

func (m InvoList) Last() *db_models.InvoItem {
	return m[len(m)-1]
}

type ImporterSource string

const (
	CsvSource  ImporterSource = "csv"
	XlsSource  ImporterSource = "xls"
	JsonSource ImporterSource = "json"
)

type ImporterQuery struct {
	Source ImporterSource        `json:"source" form:"source" schema:"source"`
	MpType db_models.OrderMpType `json:"mp_type" form:"mp_type" schema:"mp_type" binding:"required"`
	TeamID uint                  `json:"team_id" form:"team_id" schema:"team_id" binding:"required"`
}

type WDImporterQuery struct {
	*ImporterQuery
	MpID uint `json:"mp_id" form:"mp_id" schema:"mp_id" binding:"required"`
}

type importerProcessorImpl struct {
	// appctx app_iface.AppContext
	query *WDImporterQuery
	fin   order_query.Finance
	db    *gorm.DB
	agent identity_iface.Agent
	pub   streampipe.PublishProvider

	wd             *db_models.Withdrawal
	itemlists      InvoList
	adjlists       []uint
	firstOrderTime time.Time
}

// SetFilterMarketplace implements ImporterProcessor.
func (i *importerProcessorImpl) SetFilterMarketplace(mp marketplace_query.ItemQuery) error {
	var err error
	i.firstOrderTime, err = mp.FirstOrder()

	return err
}

// Unknown implements ImporterProcessor.
func (i *importerProcessorImpl) Unknown(item *db_models.InvoItem) error {
	var err error
	i.itemlists = append(i.itemlists, item)

	if item.ExternalOrderID == "" {
		err = i.db.Transaction(func(tx *gorm.DB) error {
			err := i.addOrderNotFound(tx, item)
			if err != nil {
				return err
			}

			if i.needIncrement() {
				wdquery := i.fin.DataQuery(i.agent, tx).WithdrawalByID(i.wd.ID)
				err = wdquery.IncActualAmount(item.Amount)

				if err != nil {
					return err
				}
			}

			return nil

		})
	} else {
		err = i.OrderAdjustment(item)
	}

	return err
}

// Withdrawal implements ImporterProcessor.
func (i *importerProcessorImpl) Withdrawal(item *db_models.InvoItem) error {
	var err error
	err = i.Check(item.BalanceAfter)
	if err != nil {
		return err
	}

	i.itemlists = InvoList{}
	i.adjlists = []uint{}

	// checking date time last order masuk system
	if item.TransactionDate.Before(i.firstOrderTime) {
		i.wd = nil
		return nil
	}

	// add to history wd
	err = i.db.Transaction(func(tx *gorm.DB) error {
		var err error
		// var mpquery order_query.MpItemQuery
		mpquery := order_query.NewMarketplaceQuery(i.fin, tx, i.agent).
			ByID(i.query.TeamID, i.query.MpID)
		err = mpquery.Lock()
		if err != nil {
			return err
		}
		i.wd, err = mpquery.CreateWithdrawal(item.TransactionDate, math.Abs(item.Amount), item.BalanceAfter)
		if err != nil {
			return err
		}

		return nil
	})

	return err
}

// OrderAdjustment implements ImporterProcessor.
func (i *importerProcessorImpl) OrderAdjustment(item *db_models.InvoItem) error {
	i.itemlists = append(i.itemlists, item)
	err := i.db.Transaction(func(tx *gorm.DB) error {
		query := order_query.NewOrderQuery(tx, i.agent, i.pub).ByRefID(i.query.TeamID, item.ExternalOrderID)
		err := query.Lock()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				err = i.addOrderNotFound(tx, item)
			}
			return err
		}

		adjID, err := query.LogAdjustment(item.Type, item.TransactionDate, time.Time{}, item.Amount, item.Description)
		if err != nil {
			return err
		}
		i.adjlists = append(i.adjlists, adjID)

		if i.needIncrement() {
			wdquery := i.fin.DataQuery(i.agent, tx).WithdrawalByID(i.wd.ID)
			err = wdquery.IncActualAmount(item.Amount)

			if err != nil {
				return err
			}
		}

		return nil

	})

	return err
}

// OrderFund implements ImporterProcessor.
func (i *importerProcessorImpl) OrderFund(item *db_models.InvoItem) error {
	i.itemlists = append(i.itemlists, item)

	err := i.db.Transaction(func(tx *gorm.DB) error {
		var err error
		query := order_query.NewOrderQuery(tx, i.agent, i.pub).ByRefID(i.query.TeamID, item.ExternalOrderID)
		err = query.Lock()
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				err = i.addOrderNotFound(tx, item)
			}
			return err
		}
		adjID, err := query.LogAdjustment(item.Type, item.TransactionDate, time.Time{}, item.Amount, item.Description)
		if err != nil {
			return err
		}
		i.adjlists = append(i.adjlists, adjID)

		if i.needIncrement() {
			wdquery := i.fin.DataQuery(i.agent, tx).WithdrawalByID(i.wd.ID)
			err = wdquery.IncActualAmount(item.Amount)
			if err != nil {
				return err
			}
		}

		return nil
	})

	return err
}

func (i *importerProcessorImpl) Check(sisaAmount float64) error {
	var err error
	if len(i.itemlists) == 0 {
		return err
	}

	if i.wd == nil { // get jika atasnya ada
		first := i.itemlists.First()
		wd, err := i.getNextWithdrawal(first.TransactionDate, i.itemlists.Amount())
		if err != nil {
			return err
		}

		if wd.ID != 0 {
			i.wd = wd
		}
	}

	if i.wd == nil {
		return nil
	}

	if sisaAmount != 0 {
		err = i.db.Transaction(func(tx *gorm.DB) error {
			return i.wdIncBalanceSisa(tx, sisaAmount)
		})
		if err != nil {
			return err
		}
	}

	refIDs := i.itemlists.OrderRefIDs()
	err = i.db.Transaction(func(tx *gorm.DB) error {

		refIDsQuery := order_query.NewOrderQuery(tx, i.agent, i.pub).ByRefIDs(i.query.TeamID, refIDs)
		err = refIDsQuery.Lock()
		if err != nil {
			return err
		}

		err = refIDsQuery.Completed(i.wd.At)
		if err != nil {
			return err
		}
		err = i.connectAdjToWithdrawal(tx, i.wd.ID, i.wd.At)
		if err != nil {
			return err
		}

		return err
	})

	if err != nil {
		return err
	}

	return err
}

func (i *importerProcessorImpl) addOrderNotFound(tx *gorm.DB, item *db_models.InvoItem) error {
	if i.wd == nil {
		return nil
	}
	var lostItem db_models.WdOrderNotFound

	err := tx.
		Model(&db_models.WdOrderNotFound{}).
		Where(&db_models.WdOrderNotFound{
			OrderRefID: item.ExternalOrderID,
			WdID:       i.wd.ID,
		}).
		Find(&lostItem).
		Error
	if err != nil {
		return err
	}

	if lostItem.ID != 0 {
		return nil
	}

	lostItem = db_models.WdOrderNotFound{
		OrderRefID: item.ExternalOrderID,
		WdID:       i.wd.ID,
		Amount:     item.Amount,
		At:         item.TransactionDate,
	}
	err = tx.Save(&lostItem).Error

	if err != nil {
		return err
	}

	err = tx.
		Model(&db_models.Withdrawal{}).
		Where("id = ?", i.wd.ID).
		Updates(map[string]interface{}{
			"order_not_found": tx.
				Model(&db_models.WdOrderNotFound{}).
				Select("COUNT(*)").
				Where("wd_id = ?", i.wd.ID),
		}).Error
	// updating withdrawal count

	return err
}

func (i *importerProcessorImpl) wdIncBalanceSisa(tx *gorm.DB, amount float64) error {
	if i.wd == nil {
		return errors.New("wd contain nil")
	}

	wdquery := i.fin.DataQuery(i.agent, tx).WithdrawalByID(i.wd.ID)
	return wdquery.IncActualAmount(amount)
}

func (i *importerProcessorImpl) getNextWithdrawal(tlimit time.Time, amount float64) (*db_models.Withdrawal, error) {
	var err error
	var wd db_models.Withdrawal
	next := i.fin.DataQuery(i.agent, i.db).NextWithdrawal(i.query.MpID, tlimit, amount)
	err = next.Get(&wd)
	if err != nil {
		return nil, err
	}

	return &wd, nil
}

func (i *importerProcessorImpl) connectAdjToWithdrawal(tx *gorm.DB, wdID uint, wdtime time.Time) error {
	var err error
	adjlen := len(i.adjlists)
	if adjlen == 0 {
		return nil
	}
	err = tx.Model(&db_models.WdValid{}).Where("withdrawal_id = ?", wdID).Delete(&db_models.WdValid{}).Error
	if err != nil {
		return err
	}

	// updating fund at
	err = tx.Model(&db_models.OrderAdjustment{}).Where("id IN ?", i.adjlists).Update("fund_at", wdtime).Error
	if err != nil {
		return err
	}

	for _, id := range i.adjlists {
		dd := &db_models.WdValid{
			WithdrawalID:      wdID,
			OrderAdjustmentID: id,
		}

		err = tx.
			Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "withdrawal_id"}, {Name: "order_adjustment_id"}},    // key colume
				DoUpdates: clause.AssignmentColumns([]string{"withdrawal_id", "order_adjustment_id"}), // column needed to be updated
			}).
			Create(dd).
			Error

		if err != nil {
			return fmt.Errorf("%s id order %d wd %d", err.Error(), id, wdID)
		}
	}

	err = tx.Model(&db_models.Withdrawal{}).Where("id = ?", wdID).Updates(map[string]interface{}{
		"order_valid": adjlen,
	}).Error

	if err != nil {
		return err
	}

	return nil
}

func (i *importerProcessorImpl) needIncrement() bool {
	if i.wd == nil {
		return false
	}

	if i.wd.IsNew {
		return true
	}

	if i.wd.DiffAmount == 0 {
		return false
	}

	return true
}
