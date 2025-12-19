package withdrawal_service

import (
	"net/http"

	"github.com/pdcgo/schema/services/withdrawal_iface/v1/withdrawal_ifaceconnect"
	"github.com/pdcgo/shared/custom_connect"
	"github.com/pdcgo/shared/interfaces/authorization_iface"
	"github.com/pdcgo/shared/pkg/streampipe"
	"gorm.io/gorm"
)

type RegisterHandler func()

func NewRegister(
	db *gorm.DB,
	auth authorization_iface.Authorization,
	mux *http.ServeMux,
	defaultInterceptor custom_connect.DefaultInterceptor,
	pub streampipe.PublishProvider,
) RegisterHandler {
	return func() {

		path, handler := withdrawal_ifaceconnect.NewWithdrawalServiceHandler(NewWithdrawalService(db, pub, auth), defaultInterceptor)
		mux.Handle(path, handler)
	}
}
