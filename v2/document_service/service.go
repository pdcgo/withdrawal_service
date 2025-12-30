package document_service

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"net/http"
	"time"

	"cloud.google.com/go/storage"
	"connectrpc.com/connect"
	"github.com/gabriel-vasile/mimetype"
	"github.com/pdcgo/schema/services/asset_iface/v1"
	"github.com/pdcgo/shared/db_models"
	"github.com/pdcgo/shared/interfaces/authorization_iface"
	"gorm.io/gorm"
)

type BucketConfig struct {
	WithdrawalBucket string
}

type MimeType string

const (
	JPEG         MimeType = "image/jpeg"
	JPG          MimeType = "image/jpg"
	PNG          MimeType = "image/png"
	WEBP         MimeType = "image/webp"
	PDF          MimeType = "application/pdf"
	XLS2003      MimeType = "application/vnd.ms-excel"
	XLS2007      MimeType = "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	CSV          MimeType = "text/csv"
	ZIP          MimeType = "application/zip"
	TextPlain    MimeType = "text/plain"
	TextPlainUtf MimeType = "text/plain; charset=utf-8"
)

var allowedWdMime = map[MimeType]bool{
	XLS2003:      true,
	XLS2007:      true,
	CSV:          true,
	ZIP:          true,
	TextPlain:    true,
	TextPlainUtf: true,
}

type wdDocServiceImpl struct {
	cfg    *BucketConfig
	client *storage.Client
	db     *gorm.DB
	auth   authorization_iface.Authorization
}

// Upload implements asset_ifaceconnect.WithdrawalDocumentServiceHandler.
func (w *wdDocServiceImpl) Upload(
	ctx context.Context,
	req *connect.Request[asset_iface.UploadRequest],
) (*connect.Response[asset_iface.UploadResponse], error) {
	var err error

	result := asset_iface.UploadResponse{}
	pay := req.Msg

	err = w.auth.
		AuthIdentityFromHeader(req.Header()).
		HasPermission(authorization_iface.CheckPermissionGroup{
			&db_models.Order{}: &authorization_iface.CheckPermission{
				DomainID: uint(pay.TeamId),
				Actions:  []authorization_iface.Action{authorization_iface.Update},
			},
		}).
		Err()

	mime := http.DetectContentType(pay.Content)
	if !allowedWdMime[MimeType(mime)] {
		return connect.NewResponse(&result), errors.New("invalid type")
	}

	// getting hash
	h := fnv.New64a()
	h.Write(pay.Content)
	hres := h.Sum64()

	fname, err := w.getFilename(uint(pay.TeamId), uint(pay.MarketplaceId), hres)

	if err != nil {
		return connect.NewResponse(&result), err
	}

	err = w.db.Transaction(func(tx *gorm.DB) error {
		resource := &db_models.WDResource{
			TeamID:        uint(pay.TeamId),
			MarketplaceID: uint(pay.MarketplaceId),
			Filename:      fname,
			Type:          db_models.WithdrawalResource,
			BucketType:    db_models.TempBucket,
			BucketName:    w.cfg.WithdrawalBucket,
			Path:          fmt.Sprintf("%s/%s.%s", "withdrawal_resources", fname, "xlsx"),
			CreatedAt:     time.Now(),
		}

		// uploading withdrawal
		bucket := w.client.Bucket(w.cfg.WithdrawalBucket)
		object := bucket.Object(resource.Path)
		blob := object.NewWriter(ctx)
		blob.ChunkSize = 262144

		mtype := mimetype.Detect(pay.Content)
		lencontent := len(pay.Content)
		resource.ContentLength = int64(lencontent)
		resource.MimeType = mtype.String()

		blob.ContentType = resource.MimeType
		_, err = blob.Write(pay.Content)
		blob.Close()

		if err != nil {
			return err
		}

		err = tx.Save(resource).Error
		if err != nil {
			return err
		}

		result.ResourceUri = resource.Path

		//setting to public
		acl := object.ACL()
		if err := acl.Set(ctx, storage.AllUsers, storage.RoleReader); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return connect.NewResponse(&result), err
	}

	return connect.NewResponse(&result), nil
}

func (w *wdDocServiceImpl) getFilename(teamID uint, shopID uint, hash uint64) (string, error) {
	mp := db_models.Marketplace{}
	team := db_models.Team{}
	ts := time.Now()
	tstring := ts.Format("15_04_2006_01_02")

	err := w.db.Model(&db_models.Marketplace{}).First(&mp, shopID).Error
	if err != nil {
		return "", err
	}

	err = w.db.Model(&db_models.Team{}).First(&team, teamID).Error
	if err != nil {
		return "", err
	}

	fname := fmt.Sprintf("%s_%s_%s_%d", team.TeamCode, mp.MpUsername, tstring, hash)

	return fname, nil
}

func NewWithdrawalDocumentService(
	db *gorm.DB,
	auth authorization_iface.Authorization,
	client *storage.Client,
	cfg *BucketConfig,
) *wdDocServiceImpl {
	return &wdDocServiceImpl{
		db:     db,
		auth:   auth,
		cfg:    cfg,
		client: client,
	}
}
