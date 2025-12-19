package withdrawal_service

import (
	"github.com/pdcgo/schema/services/common/v1"
	"github.com/pdcgo/schema/services/withdrawal_iface/v1"
	"github.com/pdcgo/shared/authorization"
	"github.com/pdcgo/shared/db_models"
	"gorm.io/datatypes"
)

type TaskItem struct {
	ID        uint `gorm:"primarykey"`
	AgentData datatypes.JSONType[*authorization.JwtIdentity]

	*withdrawal_iface.TaskItem
}

// func (TaskItem) TableName() string {
// 	return "task_items_v2"
// }

func (t *TaskItem) ToLegacyWDImporterQuery() *WDImporterQuery {
	var source ImporterSource = XlsSource
	switch t.Source {
	case withdrawal_iface.ImporterSource_IMPORTER_SOURCE_CSV:
		source = CsvSource
	case withdrawal_iface.ImporterSource_IMPORTER_SOURCE_JSON:
		source = JsonSource
	case withdrawal_iface.ImporterSource_IMPORTER_SOURCE_XLS:
		source = XlsSource
	}

	var mpType db_models.OrderMpType
	switch t.MpType {
	case common.MarketplaceType_MARKETPLACE_TYPE_CUSTOM:
		mpType = db_models.OrderMpCustom
	case common.MarketplaceType_MARKETPLACE_TYPE_LAZADA:
		mpType = db_models.OrderMpLazada
	case common.MarketplaceType_MARKETPLACE_TYPE_MENGANTAR:
		mpType = db_models.OrderMengantar
	case common.MarketplaceType_MARKETPLACE_TYPE_SHOPEE:
		mpType = db_models.OrderMpShopee
	case common.MarketplaceType_MARKETPLACE_TYPE_TIKTOK:
		mpType = db_models.OrderMpTiktok
	case common.MarketplaceType_MARKETPLACE_TYPE_TOKOPEDIA:
		mpType = db_models.OrderMpTokopedia
	}

	return &WDImporterQuery{
		ImporterQuery: &ImporterQuery{
			Source: source,
			MpType: mpType,
			TeamID: uint(t.TeamId),
		},
		MpID: uint(t.MpId),
	}
}
