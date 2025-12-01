package storage

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"io"
	"sync"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// MemoryStorage keeps blobs in memory. Useful for tests and local dev.
type MemoryStorage struct {
	mu    sync.RWMutex
	blobs map[string]storedBlob
}

type storedBlob struct {
	data     []byte
	mimeType string
	etag     string
}

// NewMemoryStorage constructs storage.
func NewMemoryStorage() *MemoryStorage {
	return &MemoryStorage{blobs: make(map[string]storedBlob)}
}

// Put stores the blob and returns metadata.
func (s *MemoryStorage) Put(_ context.Context, key string, data []byte, mimeType string) (domain.StoredObject, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	hash := md5.Sum(data)
	etag := hex.EncodeToString(hash[:])
	s.blobs[key] = storedBlob{data: data, mimeType: mimeType, etag: etag}
	return domain.StoredObject{
		Key:      key,
		Size:     int64(len(data)),
		MimeType: mimeType,
		ETag:     etag,
	}, nil
}

// Get returns a reader for the stored blob.
func (s *MemoryStorage) Get(_ context.Context, key string) (io.ReadCloser, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	blob, ok := s.blobs[key]
	if !ok {
		return io.NopCloser(bytes.NewReader(nil)), fmt.Errorf("blob not found")
	}
	return io.NopCloser(bytes.NewReader(blob.data)), nil
}

// Delete removes the blob.
func (s *MemoryStorage) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.blobs, key)
	return nil
}

var _ domain.ObjectStorage = (*MemoryStorage)(nil)
