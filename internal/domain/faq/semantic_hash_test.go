package faq

import "testing"

func TestSemanticHasherDeterministic(t *testing.T) {
	hasher := newSemanticHasher(8, 99)
	vector := []float32{0.1, -0.2, 0.3, 0.4}

	first, ok, err := hasher.Hash(vector)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected hash to be computed")
	}
	second, ok, err := hasher.Hash(vector)
	if err != nil {
		t.Fatalf("unexpected error on second hash: %v", err)
	}
	if !ok {
		t.Fatalf("expected hash to be computed on second attempt")
	}
	if first != second {
		t.Fatalf("expected deterministic hash got %d and %d", first, second)
	}

	hasher2 := newSemanticHasher(8, 99)
	third, ok, err := hasher2.Hash(vector)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !ok {
		t.Fatalf("expected hash to be computed on third attempt")
	}
	if first != third {
		t.Fatalf("expected hashes to match across instances, %d vs %d", first, third)
	}
}

func TestSemanticHasherHandlesEmptyVector(t *testing.T) {
	hasher := newSemanticHasher(8, 99)
	hash, ok, err := hasher.Hash(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Fatalf("expected hash flag to be false for nil vector")
	}
	if hash != 0 {
		t.Fatalf("expected zero hash for nil vector got %d", hash)
	}
}
