package withdrawal_test

import (
	"context"
	"net/http/httptest"
	"os"
	"testing"

	"cloud.google.com/go/cloudtasks/apiv2/cloudtaskspb"
	"connectrpc.com/connect"
	"github.com/golang/mock/gomock"
	"github.com/googleapis/gax-go/v2"
	"github.com/pdcgo/accounting_service/accounting_core"
	"github.com/pdcgo/accounting_service/accounting_mock"
	"github.com/pdcgo/accounting_service/revenue"
	"github.com/pdcgo/schema/services/accounting_iface/v1"
	"github.com/pdcgo/schema/services/accounting_iface/v1/accounting_iface_mock"
	"github.com/pdcgo/schema/services/common/v1"
	"github.com/pdcgo/schema/services/revenue_iface/v1/revenue_ifaceconnect"
	withdrawal_iface_v1 "github.com/pdcgo/schema/services/withdrawal_iface/v1"
	"github.com/pdcgo/schema/services/withdrawal_iface/v2"
	"github.com/pdcgo/schema/services/withdrawal_iface/v2/withdrawal_ifaceconnect"
	"github.com/pdcgo/shared/authorization/authorization_mock"
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/moretest"
	"github.com/pdcgo/shared/pkg/moretest/moretest_mock"
	"github.com/pdcgo/withdrawal_service/v2/withdrawal"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

type mockWithdrawal struct {
}

// GetTaskList implements withdrawal_ifaceconnect.WithdrawalServiceClient.
func (m *mockWithdrawal) GetTaskList(context.Context, *connect.Request[withdrawal_iface_v1.GetTaskListRequest]) (*connect.Response[withdrawal_iface_v1.GetTaskListResponse], error) {
	panic("unimplemented")
}

// HealthCheck implements withdrawal_ifaceconnect.WithdrawalServiceClient.
func (m *mockWithdrawal) HealthCheck(context.Context, *connect.Request[withdrawal_iface_v1.HealthCheckRequest]) (*connect.Response[withdrawal_iface_v1.HealthCheckResponse], error) {
	panic("unimplemented")
}

// Run implements withdrawal_ifaceconnect.WithdrawalServiceClient.
func (m *mockWithdrawal) Run(context.Context, *connect.Request[withdrawal_iface_v1.RunRequest]) (*connect.Response[withdrawal_iface_v1.RunResponse], error) {
	panic("unimplemented")
}

// Stop implements withdrawal_ifaceconnect.WithdrawalServiceClient.
func (m *mockWithdrawal) Stop(context.Context, *connect.Request[withdrawal_iface_v1.StopRequest]) (*connect.Response[withdrawal_iface_v1.StopResponse], error) {
	panic("unimplemented")
}

// SubmitWithdrawal implements withdrawal_ifaceconnect.WithdrawalServiceClient.
func (m *mockWithdrawal) SubmitWithdrawal(context.Context, *connect.Request[withdrawal_iface_v1.SubmitWithdrawalRequest]) (*connect.Response[withdrawal_iface_v1.SubmitWithdrawalResponse], error) {
	return &connect.Response[withdrawal_iface_v1.SubmitWithdrawalResponse]{}, nil
}

type mockStorage struct {
}

// GetContent implements withdrawal_service.WithdrawalStorage.
func (m *mockStorage) GetContent(ctx context.Context, uri string) ([]byte, error) {

	content, err := os.ReadFile(uri)
	return content, err

}

type mockOrderRepo struct {
	db *gorm.DB
}

// OrderByExternalID implements withdrawal_service.OrderRepo.
func (m *mockOrderRepo) OrderByExternalID(extID string) (*db_models.Order, error) {
	ord := db_models.Order{
		ID:           1,
		OrderMpTotal: 100000,
	}
	// err := m.db.
	// 	Model(&db_models.Order{}).
	// 	Where("order_ref_id = ?", extID).
	// 	Where("status != ?", db_models.OrdCancel).
	// 	Find(&ord).
	// 	Error

	return &ord, nil
}

func TestWdServiceImpl_SubmitWithdrawal(t *testing.T) {
	var db gorm.DB

	var migrate moretest.SetupFunc = func(t *testing.T) func() error {
		err := db.AutoMigrate(
			&db_models.Marketplace{},
			&db_models.Order{},
			&accounting_core.Transaction{},
			&accounting_core.TypeLabel{},
			&accounting_core.TransactionTypeLabel{},
			&accounting_core.TypeLabelDailyBalance{},
			&accounting_core.JournalEntry{},
			&accounting_core.TransactionShop{},
			&accounting_core.AccountingTag{},
			&accounting_core.TransactionTag{},
		)

		assert.Nil(t, err)

		return nil
	}

	var seed moretest.SetupFunc = func(t *testing.T) func() error {
		mps := []*db_models.Marketplace{
			{
				ID:         1,
				TeamID:     1,
				MpUsername: "asdasd",
				MpName:     "asd",
				MpType:     db_models.MpTiktok,
			},
			{
				ID:         1,
				TeamID:     1,
				MpUsername: "kukioutfit",
				MpName:     "asd",
				MpType:     db_models.MpShopee,
			},
		}

		err := db.Save(&mps).Error
		assert.Nil(t, err)

		return nil
	}

	moretest.Suite(t, "testing submit wd",
		moretest.SetupListFunc{
			moretest_mock.MockSqliteDatabase(&db),
			migrate,
			seed,
			accounting_mock.PopulateAccountKey(&db, 1),
		},
		func(t *testing.T) {
			auth := &authorization_mock.EmptyAuthorizationMock{
				AuthIdentityMock: &authorization_mock.AuthIdentityMock{
					IdentityMock: &authorization_mock.IdentityMock{
						ID: 1,
					},
				},
			}

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			expenseClient := accounting_iface_mock.NewMockAdsExpenseServiceClient(ctrl)
			expenseClient.
				EXPECT().
				AdsExCreate(t.Context(), gomock.Any()).
				Return(&connect.Response[accounting_iface.AdsExCreateResponse]{}, nil).
				AnyTimes()

			_, handler := revenue_ifaceconnect.NewRevenueServiceHandler(
				revenue.NewRevenueService(&db, auth, nil, nil, func(ctx context.Context, req *cloudtaskspb.CreateTaskRequest, opts ...gax.CallOption) error {
					return nil
				}),
			)

			ts := httptest.NewServer(handler)
			defer ts.Close()

			revclient := revenue_ifaceconnect.NewRevenueServiceClient(ts.Client(), ts.URL)

			_, handler = withdrawal_ifaceconnect.NewWithdrawalServiceHandler(
				withdrawal.NewWithdrawalService(
					&db,
					auth,
					&mockWithdrawal{},
					revclient,
					nil,
					expenseClient,
					&mockStorage{},
					&mockOrderRepo{
						db: &db,
					},
				),
			)

			wdHttp := httptest.NewServer(handler)
			defer wdHttp.Close()

			t.Run("testing tiktok wd", func(t *testing.T) {
				wdclient := withdrawal_ifaceconnect.NewWithdrawalServiceClient(wdHttp.Client(), wdHttp.URL)
				stream, err := wdclient.SubmitWithdrawal(t.Context(), &connect.Request[withdrawal_iface.SubmitWithdrawalRequest]{
					Msg: &withdrawal_iface.SubmitWithdrawalRequest{
						TeamId: 1,
						MpSubmit: &withdrawal_iface.MpSubmit{
							MpId:   1,
							MpType: common.MarketplaceType_MARKETPLACE_TYPE_TIKTOK,
						},

						Source:      withdrawal_iface_v1.ImporterSource_IMPORTER_SOURCE_XLS,
						ResourceUri: "../../test/assets/testwd/tiktok_wd_include_gmv.xlsx",
					},
				})

				assert.Nil(t, err)

				for stream.Receive() {
					stream.Msg()
					// t.Log(msg.Message)
				}
				err = stream.Err()
				if err != nil {
					t.Error(err.Error())
				}
				assert.Nil(t, err)

				t.Run("testing up kedua kali", func(t *testing.T) {
					stream, err := wdclient.SubmitWithdrawal(t.Context(), &connect.Request[withdrawal_iface.SubmitWithdrawalRequest]{
						Msg: &withdrawal_iface.SubmitWithdrawalRequest{
							TeamId: 1,
							MpSubmit: &withdrawal_iface.MpSubmit{
								MpId:   1,
								MpType: common.MarketplaceType_MARKETPLACE_TYPE_TIKTOK,
							},
							Source:      withdrawal_iface_v1.ImporterSource_IMPORTER_SOURCE_XLS,
							ResourceUri: "../../test/assets/testwd/tiktok_wd_include_gmv.xlsx",
						},
					})

					assert.Nil(t, err)

					for stream.Receive() {
						stream.Msg()
						// t.Log(msg.Message)
					}
					err = stream.Err()
					if err != nil {
						t.Error(err.Error())
					}
					assert.Nil(t, err)
				})
			})

			t.Run("testing_fian_error", func(t *testing.T) {

				wdclient := withdrawal_ifaceconnect.NewWithdrawalServiceClient(wdHttp.Client(), wdHttp.URL)
				stream, err := wdclient.SubmitWithdrawal(t.Context(), &connect.Request[withdrawal_iface.SubmitWithdrawalRequest]{
					Msg: &withdrawal_iface.SubmitWithdrawalRequest{
						TeamId: 1,
						MpSubmit: &withdrawal_iface.MpSubmit{
							MpId:   1,
							MpType: common.MarketplaceType_MARKETPLACE_TYPE_TIKTOK,
						},
						Source:      withdrawal_iface_v1.ImporterSource_IMPORTER_SOURCE_XLS,
						ResourceUri: "../../test/assets/tiktok_fian_err.xlsx",
					},
				})

				assert.Nil(t, err)

				for stream.Receive() {
					msg := stream.Msg()
					t.Log(msg.Message)
				}
				err = stream.Err()
				if err != nil {
					t.Error(err.Error())
				}
				assert.Nil(t, err)

			})

			t.Run("testing shopee wd", func(t *testing.T) {

				wdclient := withdrawal_ifaceconnect.NewWithdrawalServiceClient(wdHttp.Client(), wdHttp.URL)
				stream, err := wdclient.SubmitWithdrawal(t.Context(), &connect.Request[withdrawal_iface.SubmitWithdrawalRequest]{
					Msg: &withdrawal_iface.SubmitWithdrawalRequest{
						TeamId: 1,
						MpSubmit: &withdrawal_iface.MpSubmit{
							MpId:   2,
							MpType: common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE,
						},
						Source:      withdrawal_iface_v1.ImporterSource_IMPORTER_SOURCE_XLS,
						ResourceUri: "../../test/assets/testwd/penyesuaian.xlsx",
					},
				})

				assert.Nil(t, err)

				for stream.Receive() {
					stream.Msg()
					// t.Log(msg.Message)
				}
				err = stream.Err()
				if err != nil {
					t.Error(err.Error())
				}
				assert.Nil(t, err)

			})

			// t.Run("testing entries", func(t *testing.T) {
			// 	entries := accounting_core.JournalEntriesList{}
			// 	err = db.
			// 		Model(&accounting_core.JournalEntriesList{}).
			// 		Find(&entries).
			// 		Error

			// 	assert.Nil(t, err)

			// 	err = entries.PrintJournalEntries(&db)
			// 	assert.Nil(t, err)
			// })

		},
	)

}
