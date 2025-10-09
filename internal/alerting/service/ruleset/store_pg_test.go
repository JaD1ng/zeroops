package ruleset

import (
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
)

func TestDurationToPgInterval(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
		expected pgtype.Interval
	}{
		{
			name:     "Zero duration",
			duration: 0,
			expected: pgtype.Interval{
				Microseconds: 0,
				Days:         0,
				Months:       0,
				Valid:        true,
			},
		},
		{
			name:     "1 second",
			duration: 1 * time.Second,
			expected: pgtype.Interval{
				Microseconds: 1000000, // 1 second = 1,000,000 microseconds
				Days:         0,
				Months:       0,
				Valid:        true,
			},
		},
		{
			name:     "1 minute",
			duration: 1 * time.Minute,
			expected: pgtype.Interval{
				Microseconds: 60000000, // 1 minute = 60,000,000 microseconds
				Days:         0,
				Months:       0,
				Valid:        true,
			},
		},
		{
			name:     "1 hour",
			duration: 1 * time.Hour,
			expected: pgtype.Interval{
				Microseconds: 3600000000, // 1 hour = 3,600,000,000 microseconds
				Days:         0,
				Months:       0,
				Valid:        true,
			},
		},
		{
			name:     "1 day",
			duration: 24 * time.Hour,
			expected: pgtype.Interval{
				Microseconds: 0, // Days are stored separately
				Days:         1,
				Months:       0,
				Valid:        true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := durationToPgInterval(tt.duration)
			if got != tt.expected {
				t.Errorf("durationToPgInterval() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestPgIntervalToDuration(t *testing.T) {
	tests := []struct {
		name        string
		interval    pgtype.Interval
		expected    time.Duration
		expectError bool
	}{
		{
			name: "Valid interval - microseconds only",
			interval: pgtype.Interval{
				Microseconds: 1000000, // 1 second
				Days:         0,
				Months:       0,
				Valid:        true,
			},
			expected:    1 * time.Second,
			expectError: false,
		},
		{
			name: "Valid interval - days only",
			interval: pgtype.Interval{
				Microseconds: 0,
				Days:         1,
				Months:       0,
				Valid:        true,
			},
			expected:    24 * time.Hour,
			expectError: false,
		},
		{
			name: "Valid interval - days and microseconds",
			interval: pgtype.Interval{
				Microseconds: 1000000, // 1 second
				Days:         1,       // 1 day
				Months:       0,
				Valid:        true,
			},
			expected:    24*time.Hour + 1*time.Second,
			expectError: false,
		},
		{
			name: "Invalid interval - contains months",
			interval: pgtype.Interval{
				Microseconds: 0,
				Days:         0,
				Months:       1,
				Valid:        true,
			},
			expected:    0,
			expectError: true,
		},
		{
			name: "Invalid interval - not valid",
			interval: pgtype.Interval{
				Microseconds: 0,
				Days:         0,
				Months:       0,
				Valid:        false,
			},
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := pgIntervalToDuration(tt.interval)
			if (err != nil) != tt.expectError {
				t.Errorf("pgIntervalToDuration() error = %v, expectError %v", err, tt.expectError)
				return
			}
			if got != tt.expected {
				t.Errorf("pgIntervalToDuration() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDurationRoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{"Zero", 0},
		{"1 second", 1 * time.Second},
		{"1 minute", 1 * time.Minute},
		{"1 hour", 1 * time.Hour},
		{"1 day", 24 * time.Hour},
		{"1 day 1 hour", 25 * time.Hour},
		{"1 day 1 hour 1 minute", 25*time.Hour + 1*time.Minute},
		{"1 day 1 hour 1 minute 1 second", 25*time.Hour + 1*time.Minute + 1*time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Convert duration to pgtype.Interval
			interval := durationToPgInterval(tt.duration)

			// Convert back to duration
			got, err := pgIntervalToDuration(interval)
			if err != nil {
				t.Errorf("pgIntervalToDuration() error = %v", err)
				return
			}

			if got != tt.duration {
				t.Errorf("Round trip conversion failed: got %v, want %v", got, tt.duration)
			}
		})
	}
}
