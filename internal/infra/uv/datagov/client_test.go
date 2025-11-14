package datagov

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNormalizeRecords(t *testing.T) {
	records := []record{
		{
			Date:             "2024-07-01",
			UpdatedTimestamp: "2024-07-01T19:00:00+08:00",
			Index: []indexEntry{
				{Hour: "2024-07-01T12:00:00+08:00", Value: 5},
				{Hour: "2024-07-01T10:00:00+08:00", Value: 3},
			},
		},
		{
			Date:             "2024-07-01",
			UpdatedTimestamp: "2024-07-01T20:00:00+08:00",
			Index: []indexEntry{
				{Hour: "2024-07-01T11:00:00+08:00", Value: 4},
				{Hour: "2024-07-01T10:00:00+08:00", Value: 3}, // duplicate hour
			},
		},
	}

	series := normalizeRecords(records, "https://example.com")

	require.Equal(t, "2024-07-01", series.Date)
	require.Equal(t, "https://example.com", series.Source)
	require.Equal(t, mustParse(t, "2024-07-01T20:00:00+08:00"), series.UpdatedAt)
	require.Len(t, series.Readings, 3)
	require.Equal(t, mustParse(t, "2024-07-01T10:00:00+08:00"), series.Readings[0].Hour)
	require.Equal(t, 3.0, series.Readings[0].Value)
	require.Equal(t, mustParse(t, "2024-07-01T12:00:00+08:00"), series.Readings[2].Hour)
}

func mustParse(t *testing.T, value string) time.Time {
	t.Helper()
	ts, err := time.Parse(time.RFC3339, value)
	require.NoError(t, err)
	return ts
}
