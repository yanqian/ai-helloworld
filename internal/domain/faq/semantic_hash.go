package faq

import (
	"errors"
	"math/rand"
	"sync"
)

const (
	defaultSemanticHashPlanes = 64
	defaultSemanticHashSeed   = 1337
)

// semanticHasher converts embedding vectors into deterministic binary hashes
// using random projection planes.
type semanticHasher struct {
	mu         sync.RWMutex
	planeCount int
	seed       int64

	dims   int
	planes [][]float32
}

func newSemanticHasher(planeCount int, seed int64) *semanticHasher {
	if planeCount <= 0 {
		planeCount = defaultSemanticHashPlanes
	}
	return &semanticHasher{
		planeCount: planeCount,
		seed:       seed,
	}
}

func (h *semanticHasher) Hash(vector []float32) (uint64, bool, error) {
	if len(vector) == 0 || h == nil {
		return 0, false, nil
	}
	if err := h.ensurePlanes(len(vector)); err != nil {
		return 0, false, err
	}

	h.mu.RLock()
	planes := h.planes
	h.mu.RUnlock()

	bitCount := len(planes)
	if bitCount > 64 {
		bitCount = 64
	}

	var hash uint64
	for i, plane := range planes {
		if i >= bitCount {
			break
		}
		if dot(vector, plane) >= 0 {
			bit := 63 - i
			hash |= 1 << bit
		}
	}

	return hash, true, nil
}

func (h *semanticHasher) ensurePlanes(dims int) error {
	if dims <= 0 {
		return errors.New("semantic hasher requires positive dimension")
	}

	h.mu.RLock()
	if h.dims == dims && len(h.planes) == h.planeCount {
		h.mu.RUnlock()
		return nil
	}
	h.mu.RUnlock()

	h.mu.Lock()
	defer h.mu.Unlock()

	if h.dims == dims && len(h.planes) == h.planeCount {
		return nil
	}

	rng := rand.New(rand.NewSource(h.seed)) // deterministic planes
	planes := make([][]float32, h.planeCount)
	for i := 0; i < h.planeCount; i++ {
		plane := make([]float32, dims)
		for j := 0; j < dims; j++ {
			plane[j] = float32(rng.NormFloat64())
		}
		planes[i] = plane
	}

	h.planes = planes
	h.dims = dims
	return nil
}

func dot(a, b []float32) float64 {
	length := len(a)
	if len(b) < length {
		length = len(b)
	}
	var sum float64
	for i := 0; i < length; i++ {
		sum += float64(a[i]) * float64(b[i])
	}
	return sum
}
