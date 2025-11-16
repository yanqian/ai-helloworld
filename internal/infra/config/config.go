package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Config aggregates runtime configuration used across the service.
type Config struct {
	HTTP      HTTPConfig      `yaml:"http"`
	Summary   SummaryConfig   `yaml:"summary"`
	LLM       LLMConfig       `yaml:"llm"`
	UVAdvisor UVAdvisorConfig `yaml:"uvAdvisor"`
	FAQ       FAQConfig       `yaml:"faq"`
}

// HTTPConfig controls server level behavior.
type HTTPConfig struct {
	Address      string          `yaml:"address"`
	ReadTimeout  time.Duration   `yaml:"readTimeout"`
	WriteTimeout time.Duration   `yaml:"writeTimeout"`
	RateLimit    RateLimitConfig `yaml:"rateLimit"`
	Retry        RetryConfig     `yaml:"retry"`
}

// RateLimitConfig drives the request limiting middleware.
type RateLimitConfig struct {
	Enabled           bool `yaml:"enabled"`
	RequestsPerMinute int  `yaml:"requestsPerMinute"`
	Burst             int  `yaml:"burst"`
}

// RetryConfig configures best-effort retries for idempotent requests.
type RetryConfig struct {
	Enabled     bool          `yaml:"enabled"`
	MaxAttempts int           `yaml:"maxAttempts"`
	BaseBackoff time.Duration `yaml:"baseBackoff"`
	Exclude     []string      `yaml:"exclude"`
}

// SummaryConfig defines the heuristics for the summarizer domain.
type SummaryConfig struct {
	MaxSummaryLen int    `yaml:"maxSummaryLen"`
	MaxKeywords   int    `yaml:"maxKeywords"`
	DefaultPrompt string `yaml:"defaultPrompt"`
}

// LLMConfig contains ChatGPT/OpenAI settings.
type LLMConfig struct {
	APIKey         string  `yaml:"apiKey"`
	BaseURL        string  `yaml:"baseUrl"`
	Model          string  `yaml:"model"`
	EmbeddingModel string  `yaml:"embeddingModel"`
	Temperature    float32 `yaml:"temperature"`
}

// UVAdvisorConfig controls the UV clothing recommendation domain.
type UVAdvisorConfig struct {
	APIBaseURL string `yaml:"apiBaseUrl"`
	Prompt     string `yaml:"prompt"`
}

// FAQConfig controls the smart FAQ service behavior.
type FAQConfig struct {
	Prompt              string         `yaml:"prompt"`
	CacheTTL            time.Duration  `yaml:"cacheTtl"`
	TopRecommendations  int            `yaml:"topRecommendations"`
	SimilarityThreshold float64        `yaml:"similarityThreshold"`
	Redis               RedisConfig    `yaml:"redis"`
	Postgres            PostgresConfig `yaml:"postgres"`
}

// RedisConfig contains connection information for cache storage.
type RedisConfig struct {
	Enabled bool   `yaml:"enabled"`
	Addr    string `yaml:"addr"`
}

// PostgresConfig contains DSN and pooling settings.
type PostgresConfig struct {
	DSN      string `yaml:"dsn"`
	MaxConns int32  `yaml:"maxConns"`
	MinConns int32  `yaml:"minConns"`
}

// Load reads configuration from a YAML file and environment variables.
func Load() (*Config, error) {
	cfg := defaultConfig()

	if path := os.Getenv("CONFIG_PATH"); path != "" {
		if err := hydrateFromFile(cfg, path); err != nil {
			return nil, err
		}
	} else if _, err := os.Stat("configs/config.yaml"); err == nil {
		if err := hydrateFromFile(cfg, "configs/config.yaml"); err != nil {
			return nil, err
		}
	}

	applyEnvOverrides(cfg)

	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	return cfg, nil
}

func hydrateFromFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read config file: %w", err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse config file: %w", err)
	}
	return nil
}

func applyEnvOverrides(cfg *Config) {
	if v := os.Getenv("HTTP_ADDRESS"); v != "" {
		cfg.HTTP.Address = v
	}
	if v := os.Getenv("SUMMARY_MAX_LEN"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.Summary.MaxSummaryLen = parsed
		}
	}
	if v := os.Getenv("SUMMARY_MAX_KEYWORDS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.Summary.MaxKeywords = parsed
		}
	}
	if v := os.Getenv("SUMMARY_DEFAULT_PROMPT"); v != "" {
		cfg.Summary.DefaultPrompt = v
	}
	if v := os.Getenv("LLM_API_KEY"); v != "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("LLM_BASE_URL"); v != "" {
		cfg.LLM.BaseURL = v
	}
	if v := os.Getenv("LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("LLM_EMBEDDING_MODEL"); v != "" {
		cfg.LLM.EmbeddingModel = v
	}
	if v := os.Getenv("LLM_TEMPERATURE"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 32); err == nil {
			cfg.LLM.Temperature = float32(parsed)
		}
	}
	if v := os.Getenv("UV_API_BASE_URL"); v != "" {
		cfg.UVAdvisor.APIBaseURL = v
	}
	if v := os.Getenv("UV_PROMPT"); v != "" {
		cfg.UVAdvisor.Prompt = v
	}
	if v := os.Getenv("FAQ_PROMPT"); v != "" {
		cfg.FAQ.Prompt = v
	}
	if v := os.Getenv("FAQ_CACHE_TTL"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			cfg.FAQ.CacheTTL = parsed
		}
	}
	if v := os.Getenv("FAQ_RECOMMENDATIONS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.FAQ.TopRecommendations = parsed
		}
	}
	if v := os.Getenv("FAQ_SIMILARITY_THRESHOLD"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 64); err == nil {
			cfg.FAQ.SimilarityThreshold = parsed
		}
	}
	if v := os.Getenv("FAQ_REDIS_ENABLED"); v != "" {
		cfg.FAQ.Redis.Enabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("FAQ_REDIS_ADDR"); v != "" {
		cfg.FAQ.Redis.Addr = v
	}
	if v := os.Getenv("FAQ_POSTGRES_DSN"); v != "" {
		cfg.FAQ.Postgres.DSN = v
	}
	if v := os.Getenv("FAQ_POSTGRES_MAX_CONNS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.FAQ.Postgres.MaxConns = int32(parsed)
		}
	}
	if v := os.Getenv("FAQ_POSTGRES_MIN_CONNS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.FAQ.Postgres.MinConns = int32(parsed)
		}
	}
	if v := os.Getenv("HTTP_RATE_LIMIT_ENABLED"); v != "" {
		cfg.HTTP.RateLimit.Enabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("HTTP_RATE_LIMIT_RPM"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.HTTP.RateLimit.RequestsPerMinute = parsed
		}
	}
	if v := os.Getenv("HTTP_RATE_LIMIT_BURST"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.HTTP.RateLimit.Burst = parsed
		}
	}
	if v := os.Getenv("HTTP_RETRY_ENABLED"); v != "" {
		cfg.HTTP.Retry.Enabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("HTTP_RETRY_MAX_ATTEMPTS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.HTTP.Retry.MaxAttempts = parsed
		}
	}
	if v := os.Getenv("HTTP_RETRY_BASE_BACKOFF"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			cfg.HTTP.Retry.BaseBackoff = parsed
		}
	}
}

func defaultConfig() *Config {
	return &Config{
		HTTP: HTTPConfig{
			Address:      ":8080",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
			RateLimit: RateLimitConfig{
				Enabled:           true,
				RequestsPerMinute: 60,
				Burst:             20,
			},
			Retry: RetryConfig{
				Enabled:     true,
				MaxAttempts: 3,
				BaseBackoff: 150 * time.Millisecond,
				Exclude: []string{
					"/api/v1/summaries/stream",
				},
			},
		},
		Summary: SummaryConfig{
			MaxSummaryLen: 200,
			MaxKeywords:   5,
			DefaultPrompt: "You are an expert writing assistant that summarizes user provided text and extracts the most important keywords. Respond using the format: SUMMARY:\\n<summary>\\n\\nKEYWORDS:\\nkeyword1, keyword2, ...",
		},
		LLM: LLMConfig{
			Model:          "gpt-4o-mini",
			EmbeddingModel: "text-embedding-3-small",
			Temperature:    0.2,
		},
		UVAdvisor: UVAdvisorConfig{
			APIBaseURL: "https://api-open.data.gov.sg/v2/real-time/api/uv",
			Prompt:     "You are a UV protection stylist for Singapore. Analyze the provided UV index readings and recommend weather appropriate clothing and protection. Respond strictly as JSON with the keys summary (string), clothing (array of <=4 short tips), protection (array of <=4 short tips), and tips (array of optional reminders). Be concise yet actionable.",
		},
		FAQ: FAQConfig{
			Prompt:              "You are a helpful knowledge base assistant. Answer the user's question clearly and concisely.",
			CacheTTL:            6 * time.Hour,
			TopRecommendations:  10,
			SimilarityThreshold: 0.7,
			Redis: RedisConfig{
				Enabled: false,
				Addr:    "",
			},
			Postgres: PostgresConfig{
				DSN:      "",
				MaxConns: 4,
				MinConns: 0,
			},
		},
	}
}

// Validate ensures the configuration is safe to use.
func (c *Config) Validate() error {
	if c.HTTP.Address == "" {
		return errors.New("http.address cannot be empty")
	}
	if c.Summary.MaxSummaryLen <= 0 {
		return errors.New("summary.maxSummaryLen must be positive")
	}
	if c.Summary.MaxKeywords <= 0 {
		return errors.New("summary.maxKeywords must be positive")
	}
	if c.Summary.DefaultPrompt == "" {
		return errors.New("summary.defaultPrompt cannot be empty")
	}
	if c.UVAdvisor.APIBaseURL == "" {
		return errors.New("uvAdvisor.apiBaseUrl cannot be empty")
	}
	if c.UVAdvisor.Prompt == "" {
		return errors.New("uvAdvisor.prompt cannot be empty")
	}
	if c.FAQ.Prompt == "" {
		return errors.New("faq.prompt cannot be empty")
	}
	if c.FAQ.CacheTTL < 0 {
		return errors.New("faq.cacheTtl cannot be negative")
	}
	if c.FAQ.TopRecommendations < 0 {
		return errors.New("faq.topRecommendations cannot be negative")
	}
	if c.FAQ.SimilarityThreshold < 0 {
		return errors.New("faq.similarityThreshold must be non-negative")
	}
	if c.FAQ.Redis.Enabled && strings.TrimSpace(c.FAQ.Redis.Addr) == "" {
		return errors.New("faq.redis.addr cannot be empty when redis cache is enabled")
	}
	if strings.TrimSpace(c.LLM.EmbeddingModel) == "" {
		return errors.New("llm.embeddingModel cannot be empty")
	}
	if c.HTTP.RateLimit.Enabled {
		if c.HTTP.RateLimit.RequestsPerMinute <= 0 {
			return errors.New("http.rateLimit.requestsPerMinute must be positive")
		}
		if c.HTTP.RateLimit.Burst <= 0 {
			return errors.New("http.rateLimit.burst must be positive")
		}
	}
	if c.HTTP.Retry.Enabled {
		if c.HTTP.Retry.MaxAttempts <= 0 {
			return errors.New("http.retry.maxAttempts must be positive")
		}
		if c.HTTP.Retry.BaseBackoff <= 0 {
			return errors.New("http.retry.baseBackoff must be positive")
		}
	}
	return nil
}
