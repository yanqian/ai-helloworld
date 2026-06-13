package http

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/yanqian/ai-helloworld/internal/domain/auth"
	"github.com/yanqian/ai-helloworld/internal/domain/faq"
	"github.com/yanqian/ai-helloworld/internal/domain/summarizer"
	uploadask "github.com/yanqian/ai-helloworld/internal/domain/uploadask"
	"github.com/yanqian/ai-helloworld/internal/domain/uvadvisor"
	"github.com/yanqian/ai-helloworld/pkg/metrics"
)

func TestFrontendContractJSONFields(t *testing.T) {
	cases := []struct {
		name      string
		typ       reflect.Type
		fieldName string
		jsonName  string
	}{
		{name: "auth refresh token", typ: reflect.TypeOf(auth.LoginResponse{}), fieldName: "RefreshToken", jsonName: "refreshToken"},
		{name: "summarizer duration", typ: reflect.TypeOf(summarizer.Response{}), fieldName: "DurationMs", jsonName: "durationMs"},
		{name: "summarizer token usage", typ: reflect.TypeOf(summarizer.Response{}), fieldName: "TokenUsage", jsonName: "tokenUsage"},
		{name: "summarizer stream partial", typ: reflect.TypeOf(summarizer.StreamChunk{}), fieldName: "PartialSummary", jsonName: "partial_summary"},
		{name: "uv duration", typ: reflect.TypeOf(uvadvisor.Response{}), fieldName: "DurationMs", jsonName: "durationMs"},
		{name: "uv token usage", typ: reflect.TypeOf(uvadvisor.Response{}), fieldName: "TokenUsage", jsonName: "tokenUsage"},
		{name: "faq duration", typ: reflect.TypeOf(faq.Response{}), fieldName: "DurationMs", jsonName: "durationMs"},
		{name: "faq token usage", typ: reflect.TypeOf(faq.Response{}), fieldName: "TokenUsage", jsonName: "tokenUsage"},
		{name: "upload ask session", typ: reflect.TypeOf(uploadask.AskResponse{}), fieldName: "SessionID", jsonName: "sessionId"},
		{name: "upload ask latency", typ: reflect.TypeOf(uploadask.AskResponse{}), fieldName: "LatencyMs", jsonName: "latencyMs"},
		{name: "upload ask history tokens", typ: reflect.TypeOf(uploadask.AskResponse{}), fieldName: "UsedHistoryTokens", jsonName: "usedHistoryTokens"},
		{name: "upload ask citation document", typ: reflect.TypeOf(uploadask.ChunkSource{}), fieldName: "DocumentID", jsonName: "documentId"},
		{name: "upload ask query log session", typ: reflect.TypeOf(uploadask.QueryLog{}), fieldName: "SessionID", jsonName: "sessionId"},
		{name: "upload ask query log latency", typ: reflect.TypeOf(uploadask.QueryLog{}), fieldName: "LatencyMs", jsonName: "latencyMs"},
		{name: "token usage prompt", typ: reflect.TypeOf(metrics.TokenUsage{}), fieldName: "PromptTokens", jsonName: "promptTokens"},
		{name: "token usage total", typ: reflect.TypeOf(metrics.TokenUsage{}), fieldName: "TotalTokens", jsonName: "totalTokens"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			field, ok := tc.typ.FieldByName(tc.fieldName)
			require.True(t, ok, "%s.%s must exist", tc.typ.Name(), tc.fieldName)
			require.Equal(t, tc.jsonName, jsonTagName(field))
		})
	}
}

func jsonTagName(field reflect.StructField) string {
	tag := field.Tag.Get("json")
	for i, r := range tag {
		if r == ',' {
			return tag[:i]
		}
	}
	return tag
}
