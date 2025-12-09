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
	Auth      AuthConfig      `yaml:"auth"`
	UploadAsk UploadAskConfig `yaml:"uploadAsk"`
}

// HTTPConfig controls server level behavior.
type HTTPConfig struct {
	Address        string          `yaml:"address"`
	ReadTimeout    time.Duration   `yaml:"readTimeout"`
	WriteTimeout   time.Duration   `yaml:"writeTimeout"`
	AllowedOrigins []string        `yaml:"allowedOrigins"`
	RateLimit      RateLimitConfig `yaml:"rateLimit"`
	Retry          RetryConfig     `yaml:"retry"`
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
// TODO : support other LLM providers and for different features, use different LLMs.
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

// UploadAskConfig controls the upload-and-ask flow.
type UploadAskConfig struct {
	VectorDim       int                   `yaml:"vectorDim"`
	MaxFileMB       int                   `yaml:"maxFileMb"`
	MaxPreviewChars int                   `yaml:"maxPreviewChars"`
	Memory          UploadAskMemoryConfig `yaml:"memory"`
	Storage         UploadStorageConfig   `yaml:"storage"`
	Redis           RedisConfig           `yaml:"redis"`
	Postgres        PostgresConfig        `yaml:"postgres"`
	Worker          UploadWorkerConfig    `yaml:"worker"`
}

// UploadStorageConfig configures object storage for uploads.
type UploadStorageConfig struct {
	Endpoint  string `yaml:"endpoint"`
	AccessKey string `yaml:"accessKey"`
	SecretKey string `yaml:"secretKey"`
	Bucket    string `yaml:"bucket"`
	Region    string `yaml:"region"`
}

// UploadWorkerConfig toggles background processing.
type UploadWorkerConfig struct {
	Enabled bool `yaml:"enabled"`
}

// UploadAskMemoryConfig toggles conversational memory.
type UploadAskMemoryConfig struct {
	Enabled            bool `yaml:"enabled"`
	TopKMems           int  `yaml:"topKMems"`
	MaxHistoryTokens   int  `yaml:"maxHistoryTokens"`
	MemoryVectorDim    int  `yaml:"memoryVectorDim"`
	SummaryEveryNTurns int  `yaml:"summaryEveryNTurns"`
	PruneLimit         int  `yaml:"pruneLimit"`
}

// AuthConfig controls authentication settings.
type AuthConfig struct {
	JWTSecret       string         `yaml:"jwtSecret"`
	AccessTokenTTL  time.Duration  `yaml:"accessTokenTtl"`
	RefreshTokenTTL time.Duration  `yaml:"refreshTokenTtl"`
	Postgres        PostgresConfig `yaml:"postgres"`
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
	if v := os.Getenv("HTTP_READ_TIMEOUT"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			cfg.HTTP.ReadTimeout = parsed
		}
	}
	if v := os.Getenv("HTTP_WRITE_TIMEOUT"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			cfg.HTTP.WriteTimeout = parsed
		}
	}
	if v := os.Getenv("HTTP_ALLOWED_ORIGINS"); v != "" {
		cfg.HTTP.AllowedOrigins = splitAndTrim(v)
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
	if v := os.Getenv("UPLOADASK_VECTOR_DIM"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.VectorDim = parsed
		}
	}
	if v := os.Getenv("UPLOADASK_MAX_FILE_MB"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.MaxFileMB = parsed
		}
	}
	if v := os.Getenv("UPLOADASK_MAX_PREVIEW_CHARS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.MaxPreviewChars = parsed
		}
	}
	if v := os.Getenv("UPLOADASK_MEMORY_ENABLED"); v != "" {
		cfg.UploadAsk.Memory.Enabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("UPLOADASK_MEMORY_TOPK_MEMS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.Memory.TopKMems = parsed
		}
	}
	if v := os.Getenv("UPLOADASK_MEMORY_MAX_HISTORY_TOKENS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.Memory.MaxHistoryTokens = parsed
		}
	}
	if v := os.Getenv("UPLOADASK_MEMORY_VECTOR_DIM"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.Memory.MemoryVectorDim = parsed
		}
	}
	if v := os.Getenv("UPLOADASK_MEMORY_SUMMARY_EVERY_N_TURNS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.Memory.SummaryEveryNTurns = parsed
		}
	}
	if v := os.Getenv("UPLOADASK_MEMORY_PRUNE_LIMIT"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.Memory.PruneLimit = parsed
		}
	}
	if v := os.Getenv("UPLOADASK_STORAGE_ENDPOINT"); v != "" {
		cfg.UploadAsk.Storage.Endpoint = v
	}
	if v := os.Getenv("UPLOADASK_STORAGE_ACCESS_KEY"); v != "" {
		cfg.UploadAsk.Storage.AccessKey = v
	}
	if v := os.Getenv("UPLOADASK_STORAGE_SECRET_KEY"); v != "" {
		cfg.UploadAsk.Storage.SecretKey = v
	}
	if v := os.Getenv("UPLOADASK_STORAGE_BUCKET"); v != "" {
		cfg.UploadAsk.Storage.Bucket = v
	}
	if v := os.Getenv("UPLOADASK_STORAGE_REGION"); v != "" {
		cfg.UploadAsk.Storage.Region = v
	}
	if v := os.Getenv("UPLOADASK_POSTGRES_DSN"); v != "" {
		cfg.UploadAsk.Postgres.DSN = v
	}
	if v := os.Getenv("UPLOADASK_POSTGRES_MAX_CONNS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.Postgres.MaxConns = int32(parsed)
		}
	}
	if v := os.Getenv("UPLOADASK_POSTGRES_MIN_CONNS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.UploadAsk.Postgres.MinConns = int32(parsed)
		}
	}
	if v := os.Getenv("UPLOADASK_WORKER_ENABLED"); v != "" {
		cfg.UploadAsk.Worker.Enabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("UPLOADASK_REDIS_ENABLED"); v != "" {
		cfg.UploadAsk.Redis.Enabled = v == "1" || strings.EqualFold(v, "true")
	}
	if v := os.Getenv("UPLOADASK_REDIS_ADDR"); v != "" {
		cfg.UploadAsk.Redis.Addr = v
	}
	if v := os.Getenv("JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("AUTH_JWT_SECRET"); v != "" {
		cfg.Auth.JWTSecret = v
	}
	if v := os.Getenv("AUTH_ACCESS_TOKEN_TTL"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			cfg.Auth.AccessTokenTTL = parsed
		}
	}
	if v := os.Getenv("AUTH_REFRESH_TOKEN_TTL"); v != "" {
		if parsed, err := time.ParseDuration(v); err == nil {
			cfg.Auth.RefreshTokenTTL = parsed
		}
	}
	if v := os.Getenv("AUTH_POSTGRES_DSN"); v != "" {
		cfg.Auth.Postgres.DSN = v
	}
	if v := os.Getenv("AUTH_POSTGRES_MAX_CONNS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.Auth.Postgres.MaxConns = int32(parsed)
		}
	}
	if v := os.Getenv("AUTH_POSTGRES_MIN_CONNS"); v != "" {
		if parsed, err := strconv.Atoi(v); err == nil {
			cfg.Auth.Postgres.MinConns = int32(parsed)
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
			Address: ":8080",
			AllowedOrigins: []string{
				"*",
			},
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
					"/api/v1/auth/login",
					"/api/v1/auth/register",
					"/api/v1/auth/refresh",
					"/api/v1/upload-ask/documents",
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
				MaxConns: 10,
				MinConns: 2,
			},
		},
		Auth: AuthConfig{
			AccessTokenTTL:  time.Hour,
			RefreshTokenTTL: 24 * time.Hour,
			Postgres: PostgresConfig{
				DSN:      "",
				MaxConns: 5,
				MinConns: 1,
			},
		},
		UploadAsk: UploadAskConfig{
			VectorDim:       1536,
			MaxFileMB:       20,
			MaxPreviewChars: 240,
			Memory: UploadAskMemoryConfig{
				Enabled:            false,
				TopKMems:           3,
				MaxHistoryTokens:   800,
				MemoryVectorDim:    1536,
				SummaryEveryNTurns: 0,
				PruneLimit:         200,
			},
			Storage: UploadStorageConfig{},
			Redis: RedisConfig{
				Enabled: false,
				Addr:    "",
			},
			Postgres: PostgresConfig{
				DSN:      "",
				MaxConns: 5,
				MinConns: 1,
			},
			Worker: UploadWorkerConfig{
				Enabled: true,
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
	if c.Auth.JWTSecret == "" {
		return errors.New("auth.jwtSecret cannot be empty")
	}
	if c.Auth.AccessTokenTTL <= 0 {
		return errors.New("auth.accessTokenTtl must be positive")
	}
	if c.Auth.RefreshTokenTTL <= 0 {
		return errors.New("auth.refreshTokenTtl must be positive")
	}
	if c.UploadAsk.VectorDim <= 0 {
		return errors.New("uploadAsk.vectorDim must be positive")
	}
	if c.UploadAsk.MaxFileMB <= 0 {
		return errors.New("uploadAsk.maxFileMb must be positive")
	}
	if c.UploadAsk.MaxPreviewChars < 0 {
		return errors.New("uploadAsk.maxPreviewChars cannot be negative")
	}
	if c.UploadAsk.Memory.MaxHistoryTokens < 0 {
		return errors.New("uploadAsk.memory.maxHistoryTokens cannot be negative")
	}
	if c.UploadAsk.Memory.PruneLimit < 0 {
		return errors.New("uploadAsk.memory.pruneLimit cannot be negative")
	}
	if c.UploadAsk.Memory.Enabled {
		if c.UploadAsk.Memory.TopKMems <= 0 {
			return errors.New("uploadAsk.memory.topKMems must be positive when enabled")
		}
		if c.UploadAsk.Memory.MemoryVectorDim <= 0 {
			return errors.New("uploadAsk.memory.memoryVectorDim must be positive when memory is enabled")
		}
	}
	if c.UploadAsk.Redis.Enabled && strings.TrimSpace(c.UploadAsk.Redis.Addr) == "" {
		return errors.New("uploadAsk.redis.addr cannot be empty when uploadAsk.redis is enabled")
	}
	return nil
}

func splitAndTrim(raw string) []string {
	parts := strings.Split(raw, ",")
	var result []string
	for _, part := range parts {
		val := strings.TrimSpace(part)
		if val != "" {
			result = append(result, val)
		}
	}
	return result
}
