package summarizer

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseStructuredResponse(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		limit       int
		wantSummary string
		wantWords   []string
		wantErr     string
	}{
		{
			name:        "happy path",
			content:     "SUMMARY:\nFinal summary here.\n\nKEYWORDS:\nalpha, beta, gamma",
			limit:       2,
			wantSummary: "Final summary here.",
			wantWords:   []string{"alpha", "beta"},
		},
		{
			name:    "empty response",
			content: "",
			wantErr: "empty llm response",
		},
		{
			name:    "missing summary section",
			content: "KEYWORDS:\none, two",
			wantErr: "missing SUMMARY section",
		},
		{
			name:    "empty summary section",
			content: "SUMMARY:\n  \nKEYWORDS:\none",
			wantErr: "summary section empty",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			summary, keywords, err := parseStructuredResponse(tt.content, tt.limit)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantSummary, summary)
			require.Equal(t, tt.wantWords, keywords)
		})
	}
}

func TestSplitKeywords(t *testing.T) {
	tests := []struct {
		name  string
		raw   string
		limit int
		want  []string
	}{
		{
			name:  "limit enforced",
			raw:   "- alpha\n- beta;gamma, delta",
			limit: 2,
			want:  []string{"alpha", "beta"},
		},
		{
			name:  "no limit",
			raw:   "- alpha\n- beta;gamma, delta",
			limit: 0,
			want:  []string{"alpha", "beta", "gamma", "delta"},
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, splitKeywords(tt.raw, tt.limit))
		})
	}
}

func TestExtractSummary(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "structured body",
			content: "SUMMARY:\nConcise answer.\n\nKEYWORDS:\none, two",
			want:    "Concise answer.",
		},
		{
			name:    "plain summary",
			content: "Already summarized",
			want:    "Already summarized",
		},
		{
			name:    "empty content",
			content: "   ",
			want:    "",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, extractSummary(tt.content))
		})
	}
}

func TestFindMarker(t *testing.T) {
	tests := []struct {
		name    string
		text    string
		marker  string
		wantPos int
	}{
		{
			name:    "found",
			text:    "prefix summary: body",
			marker:  "SUMMARY:",
			wantPos: 7,
		},
		{
			name:    "not found",
			text:    "no marker",
			marker:  "summary:",
			wantPos: -1,
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.wantPos, findMarker(tt.text, tt.marker))
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "control characters removed",
			in:   " \tHello\x01world\n",
			want: "Helloworld",
		},
		{
			name: "empty remains empty",
			in:   "   ",
			want: "",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, normalize(tt.in))
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		name  string
		text  string
		limit int
		want  string
	}{
		{name: "short text unchanged", text: "short", limit: 10, want: "short"},
		{name: "limit equal three", text: "This is long", limit: 3, want: "Thi"},
		{name: "ellipse added", text: "This is long", limit: 8, want: "This..."},
		{name: "zero limit", text: "text", limit: 0, want: "text"},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			require.Equal(t, tt.want, truncate(tt.text, tt.limit))
		})
	}
}
