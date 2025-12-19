package main

import (
	"context"
	"log"

	"cloud.google.com/go/storage"
	"github.com/pdcgo/shared/configs"
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/pkg/db_tools"
	"github.com/pdcgo/shared/pkg/secret"
	"github.com/pdcgo/withdrawal_service"
	"github.com/pdcgo/withdrawal_service/datasource"
	"gorm.io/gorm"
)

func main() {
	var cfg configs.AppConfig
	var sec *secret.Secret
	var err error

	// getting configuration
	sec, err = secret.GetSecret("app_config_prod", "latest")
	if err != nil {
		panic(err)
	}
	err = sec.YamlDecode(&cfg)
	if err != nil {
		panic(err)
	}

	db, err := withdrawal_service.NewCloudDatabase(&cfg.Database)
	if err != nil {
		log.Panicln(err)
	}

	ctx := context.Background()
	client, err := storage.NewClient(ctx)
	if err != nil {
		log.Panicln(err)
	}
	defer client.Close()

	resources := []*db_models.WDResource{}
	query := db.
		Model(&db_models.WDResource{}).
		Joins("join marketplaces on marketplaces.id = wd_resources.marketplace_id").
		Where("marketplaces.mp_type = ?", "tiktok").
		Order("created_at desc")

	err = db_tools.FindInBatch(query, &resources, 100, func(tx *gorm.DB, batch int) error {
		for _, res := range resources {
			log.Println(res.Path)

			err := processWdResource(client, res)
			if err != nil {
				return err
			}
		}

		return nil
	}).Error

	if err != nil {
		log.Panicln(err)
	}

}

func processWdResource(client *storage.Client, res *db_models.WDResource) error {
	bucketName := "gudang_assets_temp"
	objectName := res.Path
	ctx := context.Background()

	// Get the object handle
	rc, err := client.Bucket(bucketName).Object(objectName).NewReader(ctx)
	if err != nil {
		log.Fatalf("Failed to create reader: %v", err)
	}
	defer rc.Close()

	xls := datasource.NewTiktokWdXls(rc)
	return xls.Iterate(ctx, func(item *db_models.InvoItem) error {
		switch item.Type {
		case db_models.AdjOrderFund:
			if item.Amount < 0 {
				log.Println(item, res.Path)
			}
		}

		return nil
	})
}
