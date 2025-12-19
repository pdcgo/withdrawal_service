package withdrawal_service

import (
	"context"
	"log/slog"

	"github.com/pdcgo/shared/yenstream"
)

type taskErr struct {
	id  uint
	err error
}

type ErrEmitter func(taskID uint, err error) error

func NewTaskErrorPipeline(store TaskStore, ctx context.Context) (ErrEmitter, func()) {
	in := make(chan *taskErr, 100)

	return func(taskID uint, err error) error {
			if err == nil {
				return nil
			}
			in <- &taskErr{
				id:  taskID,
				err: err,
			}

			return err
		}, func() {
			defer close(in)
			yenstream.
				NewRunnerContext(ctx).
				CreatePipeline(func(ctx *yenstream.RunnerContext) yenstream.Pipeline {
					source := yenstream.
						NewChannelSource(ctx, in).
						Via("saving error to item", yenstream.NewMap(ctx, func(data *taskErr) (*taskErr, error) {
							return data, store.SetErr(data.id, data.err)
						})).
						Via("debug error", yenstream.NewMap(ctx, func(data *taskErr) (*taskErr, error) {
							slog.Error(data.err.Error(), slog.Uint64("task_id", uint64(data.id)))
							return data, nil
						}))

					return source
				})
		}
}
