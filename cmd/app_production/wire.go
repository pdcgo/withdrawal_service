//go:build wireinject
// +build wireinject

package main

import (
	"net/http"

	"github.com/google/wire"
	"github.com/pdcgo/shared/configs"
	"github.com/pdcgo/shared/custom_connect"
	withdrawal_service_v1 "github.com/pdcgo/withdrawal_service"
	"github.com/pdcgo/withdrawal_service/v2"
)

func InitializeApp() (*App, error) {
	wire.Build(
		NewBucketConfig,
		configs.NewProductionConfig,
		http.NewServeMux,
		NewStorageClient,
		custom_connect.NewDefaultInterceptor,
		custom_connect.NewDefaultClientInterceptor,
		NewAccountReportServiceClient,
		NewAdsExpenseServiceClient,
		NewWithdrawalClientV1,
		NewRevenueClient,
		NewOrderClient,
		NewCache,
		NewDatabase,

		NewAuthorization,
		NewGcpPublisher,
		withdrawal_service.NewWdStorage,
		withdrawal_service.NewRegister,
		withdrawal_service_v1.NewRegister,
		NewApp,
	)
	return &App{}, nil
}
