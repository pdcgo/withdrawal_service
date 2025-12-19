package inventory_query

import (
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"gorm.io/gorm"
)

type ReturnQuery interface {
	Get(lock bool) error
	Err() error
	Cancel() error
	AddNote(orderID uint, tipe db_models.NoteType, note string) error
	SetReturnProblem() ReturnQuery
	SetResolution(resID uint) ReturnQuery
}

type InventoryQuery interface {
	// ReturnByID(teamID uint, txID uint) ReturnQuery
	ReturnByIDs(teamID uint, txIDs []uint) ReturnQuery
	// InboundItemQuery(q func(db *gorm.DB) *gorm.DB) InboundItemQuery
	// OutboundItemQuery(q func(db *gorm.DB) *gorm.DB) OutboundItemQuery

	// WarehouseTransferByInbound(teamID, inboundID uint) WarehouseTransferItem
	// WarehouseTransferByID(teamID, transferID uint) WarehouseTransferItem

	// LogHistoryTx(txID uint) LogHistoryTx

	// SkuProblem(teamID uint, txID uint) SkuProblemItemQuery

	// SkuList(skus []db_models.SkuID) SkuList
}

func NewDataQuery(tx *gorm.DB, agent identity_iface.Agent) InventoryQuery {
	return &dataQueryImpl{
		tx:    tx,
		agent: agent,
	}
}

type dataQueryImpl struct {
	tx    *gorm.DB
	agent identity_iface.Agent
}

// ReturnByIDs implements InventoryQuery.
func (d *dataQueryImpl) ReturnByIDs(teamID uint, txIDs []uint) ReturnQuery {
	return &returnByIdsImpl{
		tx:     d.tx,
		agent:  d.agent,
		teamID: teamID,
		txIDs:  txIDs,
	}
}
