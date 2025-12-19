package withdrawal_service_test

import (
	"errors"
	"testing"
	"time"

	"github.com/pdcgo/schema/services/withdrawal_iface/v1"
	"github.com/pdcgo/shared/pkg/moretest"
	"github.com/pdcgo/shared/pkg/moretest/moretest_mock"
	"github.com/pdcgo/shared/yenstream"
	"github.com/pdcgo/withdrawal_service"
	"github.com/stretchr/testify/assert"
	"gorm.io/gorm"
)

func TestStoreTask(t *testing.T) {

	var db gorm.DB

	var migrate moretest.SetupFunc = func(t *testing.T) func() error {
		db.AutoMigrate(
			&withdrawal_service.TaskItem{},
		)

		return nil
	}

	moretest.Suite(t, "test store task",
		moretest.SetupListFunc{
			moretest_mock.MockSqliteDatabase(&db),
			migrate,
		},
		func(t *testing.T) {
			moretest.Suite(t, "reprocessing task last processed lama",
				moretest.SetupListFunc{
					func(t *testing.T) func() error {
						task := withdrawal_service.TaskItem{
							TaskItem: &withdrawal_iface.TaskItem{
								TeamId:          1,
								MpId:            1,
								Status:          withdrawal_iface.TaskStatus_TASK_STATUS_PROCESS,
								Source:          withdrawal_iface.ImporterSource_IMPORTER_SOURCE_XLS,
								MpType:          1,
								ResourceUri:     "test",
								LastProcessedAt: time.Now().AddDate(0, -1, 0).Unix(),
							},
						}

						err := db.Save(&task).Error
						assert.Nil(t, err)

						tasknull := withdrawal_service.TaskItem{
							TaskItem: &withdrawal_iface.TaskItem{
								TeamId:      1,
								MpId:        1,
								Status:      withdrawal_iface.TaskStatus_TASK_STATUS_PROCESS,
								Source:      withdrawal_iface.ImporterSource_IMPORTER_SOURCE_XLS,
								MpType:      1,
								ResourceUri: "test",
								// LastProcessedAt: time.Now().AddDate(0, -1, 0).Unix(),
							},
						}

						err = db.Save(&tasknull).Error
						assert.Nil(t, err)

						return func() error {
							err = db.Delete(&task).Error
							if err != nil {
								return err
							}
							return db.Delete(&tasknull).Error
						}

					},
				},
				func(t *testing.T) {
					var count int
					yenstream.
						NewRunnerContext(t.Context()).
						CreatePipeline(func(ctx *yenstream.RunnerContext) yenstream.Pipeline {
							store := withdrawal_service.
								NewTaskSource("wd", &db, ctx, true)

							return store.Via("mapping test", yenstream.NewMap(ctx, func(item any) (any, error) {
								count += 1
								return item, nil
							}))
						})

					assert.Equal(t, 2, count)

				},
			)

			moretest.Suite(t, "test task error",
				moretest.SetupListFunc{
					func(t *testing.T) func() error {
						task := withdrawal_service.TaskItem{
							ID: 1,
						}

						err := db.Save(&task).Error
						assert.Nil(t, err)

						return func() error {
							return db.Delete(&task).Error
						}
					},
				},
				func(t *testing.T) {

					st := withdrawal_service.NewTempStore(&db)
					err := st.SetErr(1, errors.New("test dummy error"))
					assert.Nil(t, err)

					t.Run("check data", func(t *testing.T) {
						item := withdrawal_service.TaskItem{}
						err := db.Model(&withdrawal_service.TaskItem{}).Where("id = ?", 1).First(&item).Error
						assert.Nil(t, err)
						assert.Equal(t, withdrawal_iface.TaskStatus_TASK_STATUS_ERROR, item.Status)

					})
				},
			)

		},
	)

}
