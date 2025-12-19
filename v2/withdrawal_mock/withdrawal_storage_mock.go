package withdrawal_mock

import (
	"context"
	"os"

	"github.com/pdcgo/withdrawal_service/v2/withdrawal"
)

type withdrawalStorageMock struct{}

// GetContent implements withdrawal.WithdrawalStorage.
func (w *withdrawalStorageMock) GetContent(ctx context.Context, uri string) ([]byte, error) {
	return os.ReadFile(uri)
}

func NewWithdrawalStorageMock() withdrawal.WithdrawalStorage {
	return &withdrawalStorageMock{}
}
