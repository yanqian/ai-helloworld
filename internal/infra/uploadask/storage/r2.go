package storage

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// R2Storage stores objects in Cloudflare R2 via S3-compatible API.
type R2Storage struct {
	client *minio.Client
	bucket string
	logger *slog.Logger
}

// NewR2Storage constructs the storage adapter.
func NewR2Storage(endpoint, accessKey, secretKey, bucket, region string, logger *slog.Logger) (*R2Storage, error) {
	if logger == nil {
		logger = slog.Default()
	}
	cleanEndpoint := sanitizeEndpoint(endpoint)
	useSSL := strings.HasPrefix(strings.ToLower(endpoint), "https")
	client, err := minio.New(cleanEndpoint, &minio.Options{
		Creds:        credentials.NewStaticV4(accessKey, secretKey, ""),
		Secure:       useSSL,
		Region:       region,
		BucketLookup: minio.BucketLookupPath,
	})
	if err != nil {
		return nil, fmt.Errorf("init r2 client: %w", err)
	}
	return &R2Storage{client: client, bucket: bucket, logger: logger.With("component", "uploadask.storage.r2")}, nil
}

func (s *R2Storage) ensureBucket(ctx context.Context) error {
	exists, err := s.client.BucketExists(ctx, s.bucket)
	if err == nil && exists {
		return nil
	}
	err = s.client.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{})
	if err != nil && minio.ToErrorResponse(err).Code != "BucketAlreadyOwnedByYou" {
		return err
	}
	return nil
}

// Put uploads data to R2.
func (s *R2Storage) Put(ctx context.Context, key string, data []byte, mimeType string) (domain.StoredObject, error) {
	if err := s.ensureBucket(ctx); err != nil {
		return domain.StoredObject{}, err
	}
	reader := bytes.NewReader(data)
	info, err := s.client.PutObject(ctx, s.bucket, key, reader, int64(len(data)), minio.PutObjectOptions{
		ContentType:      mimeType,
		DisableMultipart: len(data) < 5*1024*1024, // small uploads as single part
	})
	if err != nil {
		return domain.StoredObject{}, err
	}
	return domain.StoredObject{
		Key:      key,
		Size:     info.Size,
		MimeType: mimeType,
		ETag:     info.ETag,
	}, nil
}

// Get fetches an object for reading.
func (s *R2Storage) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	obj, err := s.client.GetObject(ctx, s.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	// Ensure object exists before returning reader.
	if _, statErr := obj.Stat(); statErr != nil {
		return nil, statErr
	}
	return obj, nil
}

// Delete removes an object.
func (s *R2Storage) Delete(ctx context.Context, key string) error {
	return s.client.RemoveObject(ctx, s.bucket, key, minio.RemoveObjectOptions{})
}

var _ domain.ObjectStorage = (*R2Storage)(nil)

// sanitizeEndpoint removes schemes and paths to satisfy minio.New expectations.
func sanitizeEndpoint(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	raw = strings.TrimPrefix(strings.TrimPrefix(raw, "https://"), "http://")
	if strings.Contains(raw, "/") {
		parts := strings.Split(raw, "/")
		raw = parts[0]
	}
	return raw
}
