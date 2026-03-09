//go:build wireinject
// +build wireinject

package datasource_shopee

import (
	"context"
	"io"

	"github.com/google/wire"
)

func CreateValidWithdrawalHandler(ctx context.Context, files []io.ReadCloser) ([]*ShopeeWdSet, error) {
	wire.Build(
		NewDataframe,
		NewValidWithdrawal,
	)
	return nil, nil
}
