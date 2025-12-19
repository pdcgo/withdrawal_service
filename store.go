package withdrawal_service

import (
	"log/slog"
	"time"

	"github.com/pdcgo/schema/services/withdrawal_iface/v1"
	"github.com/pdcgo/shared/authorization"
	"github.com/pdcgo/shared/yenstream"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type TaskStore interface {
	Add(identity *authorization.JwtIdentity, payload *withdrawal_iface.SubmitWithdrawalRequest) error
	GetTx() *gorm.DB
	SetErr(taskID uint, err error) error
	SetFinish(taskID uint) error
	SetProcess(taskID uint) error
	EmptyTask() error
}

type tempStore struct {
	db *gorm.DB
}

// EmptyTask implements TaskStore.
func (t *tempStore) EmptyTask() error {
	return t.db.Model(&TaskItem{}).Where("status in ?", []withdrawal_iface.TaskStatus{withdrawal_iface.TaskStatus_TASK_STATUS_FINISH}).Delete(&TaskItem{}).Error
}

// SetProcess implements TaskStore.
func (t *tempStore) SetProcess(taskID uint) error {
	return t.db.Model(&TaskItem{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"is_err":            false,
		"err_message":       "",
		"status":            withdrawal_iface.TaskStatus_TASK_STATUS_PROCESS,
		"last_processed_at": time.Now().Unix(),
	}).Error
}

// SetFinish implements TaskStore.
func (t *tempStore) SetFinish(taskID uint) error {
	return t.db.Model(&TaskItem{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"is_err":      false,
		"err_message": "",
		"status":      withdrawal_iface.TaskStatus_TASK_STATUS_FINISH,
	}).Error
}

// SetErr implements TaskStore.
func (t *tempStore) SetErr(taskID uint, err error) error {
	if err == nil {
		return nil
	}
	return t.db.Model(&TaskItem{}).Where("id = ?", taskID).Updates(map[string]interface{}{
		"is_err":      true,
		"err_message": err.Error(),
		"status":      withdrawal_iface.TaskStatus_TASK_STATUS_ERROR,
	}).Error
}

// GetTx implements TaskStore.
func (t *tempStore) GetTx() *gorm.DB {
	return t.db
}

// Add implements TaskStore.
func (t *tempStore) Add(identity *authorization.JwtIdentity, payload *withdrawal_iface.SubmitWithdrawalRequest) error {
	var err error
	task := TaskItem{
		TaskItem: &withdrawal_iface.TaskItem{
			TeamId:      payload.TeamId,
			MpId:        payload.MpId,
			Status:      withdrawal_iface.TaskStatus_TASK_STATUS_WAITING,
			Source:      payload.Source,
			MpType:      payload.MpType,
			ResourceUri: payload.ResourceUri,
			CreatedAt:   time.Now().Unix(),
		},
		AgentData: datatypes.NewJSONType(identity),
	}
	err = t.db.Save(&task).Error
	return err
}

func NewTempStore(db *gorm.DB) TaskStore {

	err := db.AutoMigrate(&TaskItem{})
	if err != nil {
		panic(err)
	}

	return &tempStore{
		db: db,
	}
}

var _ yenstream.Source = (*taskSource)(nil)

type taskSource struct {
	db             *gorm.DB
	ctx            *yenstream.RunnerContext
	closeImmediate bool
	out            yenstream.NodeOut
	label          string
}

// Out implements yenstream.Source.
func (t *taskSource) Out() yenstream.NodeOut {
	return t.out
}

// SetLabel implements yenstream.Outlet.
func (t *taskSource) SetLabel(label string) {
	t.label = label
}

// Via implements yenstream.Source.
func (t *taskSource) Via(label string, pipe yenstream.Pipeline) yenstream.Pipeline {
	t.ctx.RegisterStream(label, t, pipe)
	return pipe
}

func (t *taskSource) process() {
	var err error
	out := t.out.C()
	defer close(out)

	timeoutD := time.Minute * 1
	timeout := time.NewTimer(timeoutD)
	defer timeout.Stop()
Parent:
	for {
		select {
		case <-timeout.C:
			slog.Info("source closed", slog.String("label", t.label))
			return
		case <-t.ctx.Done():
			slog.Info("source closed", slog.String("label", t.label))
			return
		default:
			task := TaskItem{
				TaskItem: &withdrawal_iface.TaskItem{},
			}

			maxProc := time.Now().Add(time.Hour * -6)

			err = t.
				db.
				Model(&TaskItem{}).
				Where("status = ? or (last_processed_at < ?  and status = ?)",
					withdrawal_iface.TaskStatus_TASK_STATUS_WAITING,
					maxProc.Unix(),
					withdrawal_iface.TaskStatus_TASK_STATUS_PROCESS,
				).
				Order("id asc").
				Find(&task).
				Error

			if err != nil {
				slog.Error(err.Error(), slog.String("label", t.label))
				return
			}

			if task.ID != 0 {
				out <- &task
			} else {
				if t.closeImmediate {
					return
				}
				continue Parent
			}

			task.Status = withdrawal_iface.TaskStatus_TASK_STATUS_PROCESS
			task.TaskItem.LastProcessedAt = time.Now().Unix()
			err = t.db.Save(&task).Error
			if err != nil {
				slog.Error(err.Error(), slog.String("label", t.label))
			}
			timeout.Reset(timeoutD)
		}
	}

}

func NewTaskSource(label string, db *gorm.DB, ctx *yenstream.RunnerContext, closeImmediate bool) *taskSource {
	source := &taskSource{
		db:             db,
		ctx:            ctx,
		out:            yenstream.NewNodeOut(ctx),
		closeImmediate: closeImmediate,
		label:          label,
	}

	ctx.AddProcess(source.process)
	return source
}
