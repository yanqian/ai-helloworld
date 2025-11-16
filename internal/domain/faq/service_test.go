package faq

import "testing"

func TestResolveSearchPlan(t *testing.T) {
	tests := []struct {
		mode SearchMode
		plan []SearchMode
	}{
		{SearchModeExact, []SearchMode{SearchModeExact}},
		{SearchModeSemanticHash, []SearchMode{SearchModeSemanticHash}},
		{SearchModeSimilarity, []SearchMode{SearchModeSimilarity}},
		{SearchModeHybrid, []SearchMode{SearchModeExact, SearchModeSimilarity}},
	}

	for _, tc := range tests {
		got := resolveSearchPlan(tc.mode)
		if len(got) != len(tc.plan) {
			t.Fatalf("mode %s: expected %v got %v", tc.mode, tc.plan, got)
		}
		for i := range got {
			if got[i] != tc.plan[i] {
				t.Fatalf("mode %s: expected plan %v got %v", tc.mode, tc.plan, got)
			}
		}
	}
}
