package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

// Config aggregates runtime configuration used across the service.
type Config struct {
	HTTP    HTTPConfig    `yaml:"http"`
	Summary SummaryConfig `yaml:"summary"`
	LLM     LLMConfig     `yaml:"llm"`
}

// HTTPConfig controls server level behavior.
type HTTPConfig struct {
	Address      string        `yaml:"address"`
	ReadTimeout  time.Duration `yaml:"readTimeout"`
	WriteTimeout time.Duration `yaml:"writeTimeout"`
}

// SummaryConfig defines the heuristics for the summarizer domain.
type SummaryConfig struct {
	MaxSummaryLen int    `yaml:"maxSummaryLen"`
	MaxKeywords   int    `yaml:"maxKeywords"`
	DefaultPrompt string `yaml:"defaultPrompt"`
}

// LLMConfig contains ChatGPT/OpenAI settings.
type LLMConfig struct {
	APIKey      string  `yaml:"apiKey"`
	BaseURL     string  `yaml:"baseUrl"`
	Model       string  `yaml:"model"`
	Temperature float32 `yaml:"temperature"`
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
	if v := os.Getenv("LLM_TEMPERATURE"); v != "" {
		if parsed, err := strconv.ParseFloat(v, 32); err == nil {
			cfg.LLM.Temperature = float32(parsed)
		}
	}
}

func defaultConfig() *Config {
	return &Config{
		HTTP: HTTPConfig{
			Address:      ":8080",
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 5 * time.Second,
		},
		Summary: SummaryConfig{
			MaxSummaryLen: 200,
			MaxKeywords:   5,
			DefaultPrompt: "You are an expert writing assistant that summarizes user provided text and extracts the most important keywords. Respond using the format: SUMMARY:\\n<summary>\\n\\nKEYWORDS:\\nkeyword1, keyword2, ...",
		},
		LLM: LLMConfig{
			Model:       "gpt-4o-mini",
			Temperature: 0.2,
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
	return nil
}
