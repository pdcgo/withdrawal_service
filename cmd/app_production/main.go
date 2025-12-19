package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"

	"cloud.google.com/go/storage"
	"connectrpc.com/connect"
	"github.com/pdcgo/schema/services/accounting_iface/v1/accounting_ifaceconnect"
	"github.com/pdcgo/schema/services/order_iface/v1/order_ifaceconnect"
	"github.com/pdcgo/schema/services/report_iface/v1/report_ifaceconnect"
	"github.com/pdcgo/schema/services/revenue_iface/v1/revenue_ifaceconnect"
	"github.com/pdcgo/schema/services/withdrawal_iface/v1/withdrawal_ifaceconnect"
	"github.com/pdcgo/shared/authorization"
	"github.com/pdcgo/shared/configs"
	"github.com/pdcgo/shared/custom_connect"
	"github.com/pdcgo/shared/db_connect"
	"github.com/pdcgo/shared/interfaces/authorization_iface"
	"github.com/pdcgo/shared/pkg/cloud_logging"
	"github.com/pdcgo/shared/pkg/streampipe"
	"github.com/pdcgo/shared/pkg/ware_cache"
	withdrawal_service_v1 "github.com/pdcgo/withdrawal_service"
	"github.com/pdcgo/withdrawal_service/v2"
	"github.com/pdcgo/withdrawal_service/v2/document_service"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
	"gorm.io/gorm"
)

func NewStorageClient() (*storage.Client, error) {
	return storage.NewClient(context.Background())
}

func NewGcpPublisher() streampipe.PublishProvider {
	return streampipe.NewGcpPublishProvider(context.Background())
}

func NewBucketConfig() *document_service.BucketConfig {
	return &document_service.BucketConfig{
		WithdrawalBucket: "gudang_assets_temp",
	}
}

func NewWithdrawalClientV1(
	cfg *configs.AppConfig,
	defaultClientInterceptor custom_connect.DefaultClientInterceptor,
) withdrawal_ifaceconnect.WithdrawalServiceClient {
	return withdrawal_ifaceconnect.NewWithdrawalServiceClient(
		http.DefaultClient,
		cfg.WithdrawalService.Endpoint,
		connect.WithGRPC(),
		defaultClientInterceptor,
	)
}

func NewAccountReportServiceClient(
	cfg *configs.AppConfig,
	defaultInterceptor custom_connect.DefaultClientInterceptor,
) report_ifaceconnect.AccountReportServiceClient {
	return report_ifaceconnect.NewAccountReportServiceClient(
		http.DefaultClient,
		cfg.AccountingService.Endpoint,
		// "http://localhost:8081",
		connect.WithGRPC(),
		defaultInterceptor,
	)
}

func NewRevenueClient(
	cfg *configs.AppConfig,
	defaultClientInterceptor custom_connect.DefaultClientInterceptor,
) revenue_ifaceconnect.RevenueServiceClient {
	return revenue_ifaceconnect.NewRevenueServiceClient(
		http.DefaultClient,
		cfg.AccountingService.Endpoint,
		// "http://localhost:8081",
		connect.WithGRPC(),
		defaultClientInterceptor,
	)
}

func NewAdsExpenseServiceClient(
	cfg *configs.AppConfig,
	defaultClientInterceptor custom_connect.DefaultClientInterceptor,
) accounting_ifaceconnect.AdsExpenseServiceClient {
	return accounting_ifaceconnect.NewAdsExpenseServiceClient(
		http.DefaultClient,
		cfg.AccountingService.Endpoint,
		connect.WithGRPC(),
		defaultClientInterceptor,
	)
}

func NewOrderClient(
	cfg *configs.AppConfig,
	defaultClientInterceptor custom_connect.DefaultClientInterceptor,
) order_ifaceconnect.OrderServiceClient {
	return order_ifaceconnect.NewOrderServiceClient(
		http.DefaultClient,
		cfg.OrderService.Endpoint,
		// "http://localhost:8083",
		connect.WithGRPC(),
		defaultClientInterceptor,
	)
}

func NewCache() (ware_cache.Cache, error) {
	return ware_cache.NewBadgerCache("/tmp/cache_withdrawals")
}

func NewDatabase(cfg *configs.AppConfig) (*gorm.DB, error) {
	return db_connect.NewProductionDatabase("withdrawal_service", &cfg.Database)
}

func NewAuthorization(
	cfg *configs.AppConfig,
	db *gorm.DB,
	cache ware_cache.Cache,
) authorization_iface.Authorization {
	return authorization.NewAuthorization(cache, db, cfg.JwtSecret)
}

type App struct {
	Run func() error
}

func NewApp(
	mux *http.ServeMux,
	wdRegisterV1 withdrawal_service_v1.RegisterHandler,
	wdRegister withdrawal_service.RegisterHandler,
	reportClient report_ifaceconnect.AccountReportServiceClient,
) *App {
	return &App{
		Run: func() error {
			cancel, err := custom_connect.InitTracer("withdrawal-service")
			if err != nil {
				return err
			}

			defer cancel(context.Background())

			wdRegister()
			wdRegisterV1()

			port := os.Getenv("PORT")
			if port == "" {
				port = "8082"
			}

			host := os.Getenv("HOST")
			listen := fmt.Sprintf("%s:%s", host, port)
			log.Println("listening on", listen)

			http.ListenAndServe(
				listen,
				// Use h2c so we can serve HTTP/2 without TLS.
				h2c.NewHandler(
					custom_connect.WithCORS(mux),
					&http2.Server{}),
			)

			return nil
		},
	}
}

func main() {
	cloud_logging.SetCloudLoggingDefault()
	app, err := InitializeApp()
	if err != nil {
		panic(err)
	}

	err = app.Run()
	if err != nil {
		panic(err)
	}
}
