package withdrawal

import (
	"context"

	"github.com/pdcgo/schema/services/accounting_iface/v1/accounting_ifaceconnect"
	"github.com/pdcgo/schema/services/order_iface/v1/order_ifaceconnect"
	"github.com/pdcgo/schema/services/revenue_iface/v1/revenue_ifaceconnect"
	wdconnectv1 "github.com/pdcgo/schema/services/withdrawal_iface/v1/withdrawal_ifaceconnect"
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/authorization_iface"
	"gorm.io/gorm"
)

type OrderRepo interface {
	OrderByExternalID(extID string) (*db_models.Order, error)
}

type WithdrawalStorage interface {
	GetContent(ctx context.Context, uri string) ([]byte, error)
}

type wdServiceImpl struct {
	db           *gorm.DB
	v1service    wdconnectv1.WithdrawalServiceClient
	auth         authorization_iface.Authorization
	rclient      revenue_ifaceconnect.RevenueServiceClient
	orderService order_ifaceconnect.OrderServiceClient
	adsService   accounting_ifaceconnect.AdsExpenseServiceClient
	storage      WithdrawalStorage
	orderRepo    OrderRepo
}

func NewWithdrawalService(
	db *gorm.DB,
	auth authorization_iface.Authorization,
	v1service wdconnectv1.WithdrawalServiceClient,
	rclient revenue_ifaceconnect.RevenueServiceClient,
	orderService order_ifaceconnect.OrderServiceClient,
	adsService accounting_ifaceconnect.AdsExpenseServiceClient,
	storage WithdrawalStorage,
	orderRepo OrderRepo,
) *wdServiceImpl {

	return &wdServiceImpl{
		db,
		v1service,
		auth,
		rclient,
		orderService,
		adsService,
		storage,
		orderRepo,
	}
}

// type RevenueRefData struct {
// 	RefType accounting_core.RefType
// 	ShopID  uint
// 	At      time.Time
// }

// func NewRevenueRefID(data *RevenueRefData) accounting_core.RefID {
// 	raw := fmt.Sprintf("%s#%d#%s", data.RefType, data.ShopID, data.At.Format("2006-01-02#1500"))
// 	return accounting_core.RefID(raw)
// }
