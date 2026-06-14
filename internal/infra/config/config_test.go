package config

import (
	"testing"
	"time"
)

func TestDefaultHTTPTimeoutsAllowLiveLLMResponses(t *testing.T) {
	cfg := defaultConfig()

	if cfg.HTTP.ReadTimeout != 10*time.Second {
		t.Fatalf("read timeout = %s, want 10s", cfg.HTTP.ReadTimeout)
	}
	if cfg.HTTP.WriteTimeout != 60*time.Second {
		t.Fatalf("write timeout = %s, want 60s", cfg.HTTP.WriteTimeout)
	}
}
