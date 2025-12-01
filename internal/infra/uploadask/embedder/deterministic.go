package embedder

import (
	"context"
	"hash/fnv"

	domain "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
)

// DeterministicEmbedder avoids network calls by hashing text into a vector.
type DeterministicEmbedder struct {
	dim int
}

// NewDeterministicEmbedder constructs the embedder.
func NewDeterministicEmbedder(dim int) *DeterministicEmbedder {
	if dim <= 0 {
		dim = 32
	}
	return &DeterministicEmbedder{dim: dim}
}

// Embed converts each text into a pseudo-random vector.
func (e *DeterministicEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vector := make([]float32, e.dim)
		hash := fnv.New64a()
		_, _ = hash.Write([]byte(text))
		seed := hash.Sum64()
		for j := 0; j < e.dim; j++ {
			seed = seed*1099511628211 + 1469598103934665603
			vector[j] = float32(seed%997) / 997.0
		}
		vectors[i] = vector
	}
	return vectors, nil
}

var _ domain.Embedder = (*DeterministicEmbedder)(nil)
