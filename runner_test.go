package withdrawal_service_test

import (
	"testing"

	"github.com/pdcgo/schema/services/common/v1"
	"github.com/pdcgo/schema/services/withdrawal_iface/v1"
	"github.com/pdcgo/shared/pkg/moretest"
	"github.com/pdcgo/shared/pkg/moretest/moretest_mock"
	"github.com/pdcgo/shared/yenstream"
	"github.com/pdcgo/withdrawal_service"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestPipeline(t *testing.T) {

	var db gorm.DB
	moretest.Suite(t, "testing stream", moretest.SetupListFunc{
		moretest_mock.MockSqliteDatabase(&db),
		func(t *testing.T) func() error {
			err := db.AutoMigrate(
				withdrawal_service.TaskItem{},
			)
			assert.Nil(t, err)
			return nil
		},
		func(t *testing.T) func() error {
			datas := []*withdrawal_service.TaskItem{
				{
					TaskItem: &withdrawal_iface.TaskItem{
						TeamId:      1,
						MpId:        1,
						Status:      withdrawal_iface.TaskStatus_TASK_STATUS_WAITING,
						Source:      withdrawal_iface.ImporterSource_IMPORTER_SOURCE_XLS,
						MpType:      common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE,
						ResourceUri: "withdrawal_resources/99_dreamoraoutfit_08_16_2025_03_06.xlsx",
					},
				},
				{
					TaskItem: &withdrawal_iface.TaskItem{
						TeamId:      1,
						MpId:        1,
						Status:      withdrawal_iface.TaskStatus_TASK_STATUS_WAITING,
						Source:      withdrawal_iface.ImporterSource_IMPORTER_SOURCE_XLS,
						MpType:      common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE,
						ResourceUri: "withdrawal_resources/99_dreamoraoutfit_08_16_2025_03_06.xlsx",
					},
				},
				{
					TaskItem: &withdrawal_iface.TaskItem{
						TeamId:      1,
						MpId:        1,
						Status:      withdrawal_iface.TaskStatus_TASK_STATUS_WAITING,
						Source:      withdrawal_iface.ImporterSource_IMPORTER_SOURCE_XLS,
						MpType:      common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE,
						ResourceUri: "withdrawal_resources/99_dreamoraoutfit_08_16_2025_03_06.xlsx",
					},
				},
				{
					TaskItem: &withdrawal_iface.TaskItem{
						TeamId:      1,
						MpId:        1,
						Status:      withdrawal_iface.TaskStatus_TASK_STATUS_WAITING,
						Source:      withdrawal_iface.ImporterSource_IMPORTER_SOURCE_XLS,
						MpType:      common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE,
						ResourceUri: "withdrawal_resources/99_dreamoraoutfit_08_16_2025_03_06.xlsx",
					},
				},
				{
					TaskItem: &withdrawal_iface.TaskItem{
						TeamId:      1,
						MpId:        1,
						Status:      withdrawal_iface.TaskStatus_TASK_STATUS_WAITING,
						Source:      withdrawal_iface.ImporterSource_IMPORTER_SOURCE_XLS,
						MpType:      common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE,
						ResourceUri: "withdrawal_resources/99_dreamoraoutfit_08_16_2025_03_06.xlsx",
					},
				},
				{
					TaskItem: &withdrawal_iface.TaskItem{
						TeamId:      1,
						MpId:        1,
						Status:      withdrawal_iface.TaskStatus_TASK_STATUS_WAITING,
						Source:      withdrawal_iface.ImporterSource_IMPORTER_SOURCE_XLS,
						MpType:      common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE,
						ResourceUri: "withdrawal_resources/99_dreamoraoutfit_08_16_2025_03_06.xlsx",
					},
				},
			}

			err := db.Save(&datas).Error
			assert.Nil(t, err)
			return nil
		},
	}, func(t *testing.T) {
		ctx := t.Context()

		yenstream.NewRunnerContext(ctx).CreatePipeline(
			func(ctx *yenstream.RunnerContext) yenstream.Pipeline {

				source := withdrawal_service.NewTaskSource("task_source", &db, ctx, true).
					Via("logging", yenstream.NewMap(ctx, func(data *withdrawal_service.TaskItem) (*withdrawal_service.TaskItem, error) {
						return data, nil
					}))
				return source
			},
		)
	})

}
