package withdrawal_service

import (
	_ "github.com/GoogleCloudPlatform/cloudsql-proxy/proxy/dialers/postgres"
	"github.com/pdcgo/shared/configs"
	"github.com/pdcgo/shared/pkg/gorm_commenter"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func NewCloudDatabase(cfg *configs.DatabaseConfig) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.New(postgres.Config{
		DriverName: "cloudsqlpostgres",
		DSN:        cfg.ToDsn("withdrawal_service"),
	}), &gorm.Config{})
	if err != nil {
		return db, err
	}

	sqldb, err := db.DB()
	if err != nil {
		return db, err
	}

	sqldb.SetMaxOpenConns(2)
	db.Use(gorm_commenter.NewCommentClausePlugin())

	return db, err

}
