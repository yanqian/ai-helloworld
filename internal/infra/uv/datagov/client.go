package datagov

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/yanqian/ai-helloworld/internal/domain/uvadvisor"
)

const defaultBaseURL = "https://api-open.data.gov.sg/v2/real-time/api/uv"

// Client fetches UV values from data.gov.sg.
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient builds an API client.
func NewClient(baseURL string) *Client {
	url := strings.TrimSpace(baseURL)
	if url == "" {
		url = defaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(url, "/"),
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Fetch retrieves UV series for a given date.
func (c *Client) Fetch(ctx context.Context, date string) (uvadvisor.UVSeries, error) {
	endpoint := c.baseURL
	if strings.TrimSpace(date) != "" {
		endpoint = fmt.Sprintf("%s?date=%s", c.baseURL, url.QueryEscape(date))
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return uvadvisor.UVSeries{}, fmt.Errorf("build uv request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return uvadvisor.UVSeries{}, fmt.Errorf("uv request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		payload, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<10))
		return uvadvisor.UVSeries{}, fmt.Errorf("uv request error: status=%d body=%s", resp.StatusCode, string(payload))
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return uvadvisor.UVSeries{}, fmt.Errorf("read uv response: %w", err)
	}

	var raw apiResponse
	if err := json.Unmarshal(body, &raw); err != nil {
		return uvadvisor.UVSeries{}, fmt.Errorf("decode uv response: %w", err)
	}

	if raw.Code != 0 {
		return uvadvisor.UVSeries{}, fmt.Errorf("uv api error: %s", raw.ErrorMsg)
	}

	series := normalizeRecords(raw.Data.Records, c.baseURL)
	series.RawJSON = body
	return series, nil
}

type apiResponse struct {
	Code     int       `json:"code"`
	ErrorMsg string    `json:"errorMsg"`
	Data     apiData   `json:"data"`
	APIInfo  apiStatus `json:"api_info"`
}

type apiData struct {
	Records []record `json:"records"`
}

type apiStatus struct {
	Status string `json:"status"`
}

type record struct {
	Date             string       `json:"date"`
	Timestamp        string       `json:"timestamp"`
	UpdatedTimestamp string       `json:"updatedTimestamp"`
	Index            []indexEntry `json:"index"`
}

type indexEntry struct {
	Hour  string  `json:"hour"`
	Value float64 `json:"value"`
}

func normalizeRecords(records []record, source string) uvadvisor.UVSeries {
	points := make([]uvadvisor.UVSample, 0, len(records)*2)
	seen := make(map[string]struct{})
	var (
		date    string
		updated time.Time
	)

	for _, rec := range records {
		if date == "" && rec.Date != "" {
			date = rec.Date
		}
		if ts := parseTime(rec.UpdatedTimestamp); !ts.IsZero() && ts.After(updated) {
			updated = ts
		}
		for _, idx := range rec.Index {
			if _, ok := seen[idx.Hour]; ok {
				continue
			}
			seen[idx.Hour] = struct{}{}

			ts := parseTime(idx.Hour)
			if ts.IsZero() {
				continue
			}
			points = append(points, uvadvisor.UVSample{
				Hour:  ts,
				Value: idx.Value,
			})
		}
	}

	sort.Slice(points, func(i, j int) bool {
		return points[i].Hour.Before(points[j].Hour)
	})

	return uvadvisor.UVSeries{
		Date:      date,
		Readings:  points,
		UpdatedAt: updated,
		Source:    source,
	}
}

func parseTime(value string) time.Time {
	if strings.TrimSpace(value) == "" {
		return time.Time{}
	}
	ts, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}
	}
	return ts
}
