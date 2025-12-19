package withdrawal_service

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"cloud.google.com/go/storage"
	"connectrpc.com/connect"
	"github.com/pdcgo/schema/services/withdrawal_iface/v1"
	"github.com/pdcgo/shared/authorization"
	"github.com/pdcgo/shared/interfaces/authorization_iface"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"github.com/pdcgo/shared/pkg/streampipe"
	"gorm.io/gorm"
)

type wdServiceImpl struct {
	auth      authorization_iface.Authorization
	emitter   ErrEmitter
	db        *gorm.DB
	store     TaskStore
	mut       sync.Mutex
	ctx       context.Context
	cancelCtx context.CancelFunc
	pub       streampipe.PublishProvider
}

// GetTaskList implements withdrawal_ifaceconnect.WithdrawalServiceHandler.
func (w *wdServiceImpl) GetTaskList(
	ctx context.Context,
	req *connect.Request[withdrawal_iface.GetTaskListRequest],
) (*connect.Response[withdrawal_iface.GetTaskListResponse], error) {
	var err error
	result := withdrawal_iface.GetTaskListResponse{
		Items: []*withdrawal_iface.TaskItem{},
	}

	pay := req.Msg

	tx := w.store.GetTx()
	query := tx.
		Model(&TaskItem{}).
		Where("team_id = ?", pay.TeamId).
		Order("id desc")

	if pay.Status != withdrawal_iface.TaskStatus_TASK_STATUS_UNSPECIFIED {
		query = query.Where("status = ?", pay.Status)
	}

	err = query.
		Find(&result.Items).
		Error

	return connect.NewResponse(&result), err
}

// HealthCheck implements withdrawal_ifaceconnect.WithdrawalServiceHandler.
func (w *wdServiceImpl) HealthCheck(context.Context, *connect.Request[withdrawal_iface.HealthCheckRequest]) (*connect.Response[withdrawal_iface.HealthCheckResponse], error) {
	result := withdrawal_iface.HealthCheckResponse{}

	return connect.NewResponse(&result), nil
}

// Run implements withdrawal_ifaceconnect.WithdrawalServiceHandler.
func (w *wdServiceImpl) Run(
	ctx context.Context,
	req *connect.Request[withdrawal_iface.RunRequest],
) (*connect.Response[withdrawal_iface.RunResponse], error) {
	var result withdrawal_iface.RunResponse

	if !w.mut.TryLock() {
		return connect.NewResponse(&result), errors.New("still_running")
	}

	w.ctx, w.cancelCtx = context.WithCancel(context.Background())
	go func() {
		defer w.mut.Unlock()
		client, err := storage.NewClient(w.ctx)
		if err != nil {
			slog.Error(err.Error())
			return

		}
		defer client.Close()
		NewRunner(w.ctx, w.db, w.store, w.pub, client).
			Run(w.emitter)
	}()

	return connect.NewResponse(&result), nil
}

// Stop implements withdrawal_ifaceconnect.WithdrawalServiceHandler.
func (w *wdServiceImpl) Stop(
	ctx context.Context,
	req *connect.Request[withdrawal_iface.StopRequest],
) (*connect.Response[withdrawal_iface.StopResponse], error) {
	w.cancelCtx()
	return connect.NewResponse(&withdrawal_iface.StopResponse{}), nil
}

// SubmitWithdrawal implements withdrawal_ifaceconnect.WithdrawalServiceHandler.
func (w *wdServiceImpl) SubmitWithdrawal(
	ctx context.Context,
	req *connect.Request[withdrawal_iface.SubmitWithdrawalRequest],
) (*connect.Response[withdrawal_iface.SubmitWithdrawalResponse], error) {
	var err error
	var result withdrawal_iface.SubmitWithdrawalResponse

	pay := req.Msg

	identity := w.
		auth.
		AuthIdentityFromHeader(req.Header())

	agent := identity.Identity()

	err = identity.
		Err()
	if err != nil {
		return connect.NewResponse(&result), err
	}

	err = w.store.Add(&authorization.JwtIdentity{
		UserID:    agent.IdentityID(),
		From:      "withdrawal_service",
		UserAgent: identity_iface.ImporterAgent,
	}, pay)
	if err != nil {
		return connect.NewResponse(&result), err
	}

	w.Run(ctx, &connect.Request[withdrawal_iface.RunRequest]{})
	return connect.NewResponse(&result), err
}

func NewWithdrawalService(
	db *gorm.DB,
	pub streampipe.PublishProvider,
	auth authorization_iface.Authorization) *wdServiceImpl {
	store := NewTempStore(db)

	emitter, run_pipe := NewTaskErrorPipeline(store, context.Background())
	go run_pipe()

	return &wdServiceImpl{
		db:      db,
		store:   store,
		pub:     pub,
		emitter: emitter,
		auth:    auth,
	}
}
