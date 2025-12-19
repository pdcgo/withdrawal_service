package withdrawal_service

import (
	"net/http"

	"cloud.google.com/go/storage"
	"github.com/pdcgo/schema/services/accounting_iface/v1/accounting_ifaceconnect"
	"github.com/pdcgo/schema/services/asset_iface/v1/asset_ifaceconnect"
	"github.com/pdcgo/schema/services/order_iface/v1/order_ifaceconnect"
	"github.com/pdcgo/schema/services/revenue_iface/v1/revenue_ifaceconnect"
	withdrawal_ifaceconnect_v1 "github.com/pdcgo/schema/services/withdrawal_iface/v1/withdrawal_ifaceconnect"
	"github.com/pdcgo/schema/services/withdrawal_iface/v2/withdrawal_ifaceconnect"
	"github.com/pdcgo/shared/custom_connect"
	"github.com/pdcgo/shared/interfaces/authorization_iface"
	"github.com/pdcgo/withdrawal_service/v2/document_service"
	"github.com/pdcgo/withdrawal_service/v2/withdrawal"
	"gorm.io/gorm"
)

type RegisterHandler func()

func NewRegister(
	bucketConfig *document_service.BucketConfig,
	db *gorm.DB,
	auth authorization_iface.Authorization,
	mux *http.ServeMux,
	storageClient *storage.Client,
	storage withdrawal.WithdrawalStorage,
	wdclientv1 withdrawal_ifaceconnect_v1.WithdrawalServiceClient,
	rclient revenue_ifaceconnect.RevenueServiceClient,
	orderService order_ifaceconnect.OrderServiceClient,
	adsService accounting_ifaceconnect.AdsExpenseServiceClient,
	defaultInterceptor custom_connect.DefaultInterceptor,
) RegisterHandler {

	return func() {

		path, handler := withdrawal_ifaceconnect.NewWithdrawalServiceHandler(
			withdrawal.NewWithdrawalService(
				db,
				auth,
				wdclientv1,
				rclient,
				orderService,
				adsService,
				storage,
				NewOrderRepo(db),
			),
			defaultInterceptor,
		)
		mux.Handle(path, handler)

		path, handler = asset_ifaceconnect.NewWithdrawalDocumentServiceHandler(
			document_service.NewWithdrawalDocumentService(db, auth, storageClient, bucketConfig), defaultInterceptor)
		mux.Handle(path, handler)
	}
}
