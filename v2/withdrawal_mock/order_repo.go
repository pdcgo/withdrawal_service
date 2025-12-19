package withdrawal_mock

import "github.com/pdcgo/shared/db_models"

type orderRepoMockImpl struct {
	OrderByExternalIDMock func(extID string) (*db_models.Order, error)
}

// OrderByExternalID implements withdrawal.OrderRepo.
func (o *orderRepoMockImpl) OrderByExternalID(extID string) (*db_models.Order, error) {
	return o.OrderByExternalIDMock(extID)
}

func NewOrderRepoMock() *orderRepoMockImpl {
	return &orderRepoMockImpl{}
}
