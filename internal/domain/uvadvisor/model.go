package uvadvisor

import "time"

// Request captures the payload accepted by the UV advisor service.
type Request struct {
	Date string `json:"date"`
}

// Response is serialized back to API consumers.
type Response struct {
	Date          string    `json:"date"`
	Category      string    `json:"category"`
	MaxUV         float64   `json:"maxUv"`
	PeakHour      string    `json:"peakHour"`
	Source        string    `json:"source"`
	Summary       string    `json:"summary"`
	Clothing      []string  `json:"clothing"`
	Protection    []string  `json:"protection"`
	Tips          []string  `json:"tips"`
	Readings      []Reading `json:"readings"`
	DataTimestamp string    `json:"dataTimestamp"`
}

// Reading models a normalized UV point used by the frontend.
type Reading struct {
	Hour  string  `json:"hour"`
	Value float64 `json:"value"`
}

// UVSeries represents a day worth of UV readings fetched from upstream.
type UVSeries struct {
	Date       string
	Readings   []UVSample
	UpdatedAt  time.Time
	Source     string
	Confidence string
	RawJSON    []byte
}

// UVSample is an individual hourly UV value.
type UVSample struct {
	Hour  time.Time
	Value float64
}

// Config wires runtime dependencies for the advisor domain.
type Config struct {
	Model       string
	Temperature float32
	Prompt      string
	SourceURL   string
}
