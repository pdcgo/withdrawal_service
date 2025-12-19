package withdrawal_service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"github.com/pdcgo/schema/services/common/v1"
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/identity_iface"
	"github.com/pdcgo/shared/pkg/streampipe"
	"github.com/pdcgo/shared/yenstream"
	"github.com/pdcgo/withdrawal_service/datasource"
	"github.com/pdcgo/withdrawal_service/marketplace_query"
	"github.com/pdcgo/withdrawal_service/order_query"
	"gorm.io/gorm"
)

type runner struct {
	ctx    context.Context
	store  TaskStore
	db     *gorm.DB
	pub    streampipe.PublishProvider
	client *storage.Client
}

func NewRunner(ctx context.Context, db *gorm.DB, store TaskStore, pub streampipe.PublishProvider, client *storage.Client) *runner {
	return &runner{
		ctx:    ctx,
		store:  store,
		db:     db,
		pub:    pub,
		client: client,
	}
}

type WdImporterIterate interface {
	GetShopUsername() (string, error)
	GetRefIDs() (datasource.OrderRefList, error)
	Iterate(ctx context.Context, handler func(item *db_models.InvoItem) error) error
}

type WdPipeParam struct {
	ctx         context.Context
	importer    WdImporterIterate
	processor   ImporterProcessor
	task        *TaskItem
	agent       identity_iface.Agent
	marketplace marketplace_query.ItemQuery
	// filecontent []byte
}

func (r *runner) runKeepAlive(ctx context.Context) {
	tick := time.NewTicker(time.Second * 10)
	defer tick.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			_, err := http.Get(getEndpoint())
			slog.Info("send keep alive")
			if err != nil {
				slog.Error(err.Error(), slog.String("function", "keep_alive"))
			}

		}
	}

}

func (r *runner) Run(errEmitter ErrEmitter) {
	slog.Info("starting withdrawal runner")

	// untuk keep alive appengine standard environtment
	aliveCtx, cancel := context.WithCancel(r.ctx)
	defer cancel()
	go r.runKeepAlive(aliveCtx)

	runCtx := yenstream.NewRunnerContext(r.ctx)
	runCtx.
		CreatePipeline(func(ctx *yenstream.RunnerContext) yenstream.Pipeline {
			source := NewTaskSource(
				"task_source",
				r.store.GetTx(),
				ctx,
				false,
			).
				Via("Set Process", yenstream.NewMap(ctx, func(task *TaskItem) (*TaskItem, error) {
					slog.Info("processing task", slog.String("resource", task.ResourceUri))
					err := r.store.SetProcess(task.ID)
					return task, errEmitter(task.ID, err)
				})).
				Via("initializing processor", yenstream.NewMap(ctx, func(item *TaskItem) (*WdPipeParam, error) {
					var err error
					var importer WdImporterIterate

					// getting file
					file, err := r.client.Bucket("gudang_assets_temp").Object(item.ResourceUri).NewReader(r.ctx)
					if err != nil {
						return nil, errEmitter(item.ID, err)
					}

					data, err := io.ReadAll(file)
					if err != nil {
						return nil, errEmitter(item.ID, err)
					}

					agent := NewV2ImporterAgent(item.AgentData.Data())
					importer, err = r.createImporter(item.MpType, data)

					var processor ImporterProcessor = NewImporterProcessor(
						r.db,
						order_query.NewFinance(agent, r.db),
						r.ctx,
						item.ToLegacyWDImporterQuery(),
						agent,
						r.pub,
					)

					return &WdPipeParam{
						ctx:       r.ctx,
						importer:  importer,
						task:      item,
						agent:     agent,
						processor: processor,
					}, errEmitter(item.ID, err)
				})).
				Via("check marketplace", yenstream.NewMap(ctx, func(data *WdPipeParam) (*WdPipeParam, error) {
					var err error
					query := data.task.ToLegacyWDImporterQuery()
					importer := data.importer
					mpquery := marketplace_query.NewMarketplaceQuery(r.db, data.agent)
					item := data.task

					var marketplace marketplace_query.ItemQuery
					switch query.MpType {
					case db_models.OrderMpShopee:
						username, err := importer.GetShopUsername()
						if err != nil {
							return data, errEmitter(item.ID, err)
						}
						marketplace = mpquery.ByUsername(query.TeamID, db_models.MpShopee, username)
						_, err = marketplace.Get()
						if err != nil {
							if errors.Is(err, marketplace_query.ErrMarketplaceNotFound) {
								return data, errEmitter(item.ID, fmt.Errorf("marketplace dengan username %s tidak ada", username))
							}

							return data, errEmitter(item.ID, err)
						}

					case db_models.OrderMpTiktok:
						marketplace = mpquery.ByID(query.TeamID, query.MpID)

					case db_models.OrderMengantar:
						marketplace = mpquery.ByID(query.TeamID, query.MpID)
					}

					// initiating asset and history flow
					err = marketplace.CheckBankAccount()
					if err != nil {
						return data, errEmitter(item.ID, err)
					}

					err = marketplace.CheckHoldAsset()
					if err != nil {
						return data, errEmitter(item.ID, err)
					}

					data.marketplace = marketplace
					return data, nil
				})).
				Via("checking order marketplace sudah benar", yenstream.NewMap(ctx, func(data *WdPipeParam) (*WdPipeParam, error) {
					query := data.task.ToLegacyWDImporterQuery()
					marketplace := data.marketplace
					importer := data.importer
					processor := data.processor
					item := data.task

					refids, err := importer.GetRefIDs()
					if err != nil {
						return data, errEmitter(item.ID, err)
					}

					// checking order marketplace benar
					refIDsQuery := order_query.NewOrderQuery(r.db, data.agent, r.pub).ByRefIDs(query.TeamID, refids)
					ordersMeta := refIDsQuery.HaveMarketplace(query.MpID)

					switch query.MpType {
					case db_models.OrderMpShopee:
						if ordersMeta.InvalidCount != 0 {
							market, err := marketplace.Get()
							if err != nil {
								return data, errEmitter(item.ID, err)
							}
							err = refIDsQuery.ChangeMarketplace(market.ID)
							if err != nil {
								return data, errEmitter(item.ID, err)
							}
						}
					case db_models.OrderMpTiktok:
						err = ordersMeta.GetError()
						if err != nil {
							return data, errEmitter(item.ID, err)
						}
					}

					err = processor.SetFilterMarketplace(marketplace)
					if err != nil {
						return data, errEmitter(item.ID, err)
					}

					return data, nil
				})).
				Via("starting iter update withdrawal", yenstream.NewMap(ctx, func(data *WdPipeParam) (*WdPipeParam, error) {

					res, err := r.iterateWithdrawal(data)
					if err != nil {
						return res, errEmitter(data.task.ID, err)
					}
					return res, nil
				})).
				Via("Set Finish", yenstream.NewMap(ctx, func(data *WdPipeParam) (*WdPipeParam, error) {
					err := r.store.SetFinish(data.task.ID)
					return data, errEmitter(data.task.ID, err)
				})).
				Via("log pipe", yenstream.NewMap(ctx, func(data *WdPipeParam) (*WdPipeParam, error) {
					slog.Info(fmt.Sprintf("%s processed", data.task.ResourceUri))
					return data, nil
				}))

			return source
		})

	err := r.store.EmptyTask()
	if err != nil {
		panic(err)
	}
	slog.Info("close runner withdrawal")
}

func (r *runner) createImporter(tipe common.MarketplaceType, data []byte) (WdImporterIterate, error) {
	var err error
	var importer WdImporterIterate

	switch tipe {
	case common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE:
		importer = datasource.NewShopeeWdXls(io.NopCloser(bytes.NewReader(data)))
	case common.MarketplaceType_MARKETPLACE_TYPE_TIKTOK:
		importer = datasource.NewTiktokWdXls(io.NopCloser(bytes.NewReader(data)))
	case common.MarketplaceType_MARKETPLACE_TYPE_MENGANTAR:
		importer = datasource.NewMengantarWdCsv(io.NopCloser(bytes.NewReader(data)))
	default:
		return importer, fmt.Errorf("%s not supported", tipe)
	}

	return importer, err
}

func (r *runner) iterateWithdrawal(data *WdPipeParam) (*WdPipeParam, error) {
	importer := data.importer
	processor := data.processor

	err := importer.Iterate(data.ctx, func(item *db_models.InvoItem) error {
		var err error
		switch item.Type {
		case db_models.AdjOrderFund:
			err = processor.OrderFund(item)
		case db_models.AdjCommision, db_models.AdjLostCompensation, db_models.AdjCompensation:
			err = processor.OrderAdjustment(item)
		case db_models.AdjUnknown, db_models.AdjUnknownAdj:
			err = processor.Unknown(item)

		case db_models.AdjFund:
			err = processor.Withdrawal(item)
		}

		if err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return data, err
	}

	err = processor.Check(0.00)
	if err != nil {
		return data, err
	}

	return data, nil

}
