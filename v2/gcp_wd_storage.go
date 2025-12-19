package withdrawal_service

import (
	"context"
	"io"

	"cloud.google.com/go/storage"
	"github.com/pdcgo/withdrawal_service/v2/document_service"
	"github.com/pdcgo/withdrawal_service/v2/withdrawal"
)

type wdStorageImpl struct {
	client *storage.Client
	cfg    *document_service.BucketConfig
}

// GetContent implements WithdrawalStorage.
func (w *wdStorageImpl) GetContent(ctx context.Context, uri string) ([]byte, error) {
	var hasil []byte
	var err error

	// getting file
	file, err := w.client.Bucket(w.cfg.WithdrawalBucket).Object(uri).NewReader(ctx)
	if err != nil {
		return hasil, err
	}

	hasil, err = io.ReadAll(file)
	if err != nil {
		return hasil, err
	}
	return hasil, err
}

func NewWdStorage(client *storage.Client, cfg *document_service.BucketConfig) withdrawal.WithdrawalStorage {
	return &wdStorageImpl{
		client: client,
		cfg:    cfg,
	}
}
