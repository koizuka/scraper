package scraper

import (
	"os"
	"testing"
	"time"
)

func TestGetCIMinTimeout(t *testing.T) {
	tests := []struct {
		name      string
		ciValue   string
		requested time.Duration
		want      time.Duration
	}{
		{
			name:      "local environment keeps requested timeout",
			ciValue:   "",
			requested: 30 * time.Second,
			want:      30 * time.Second,
		},
		{
			name:      "CI enforces minimum timeout",
			ciValue:   "true",
			requested: 10 * time.Second,
			want:      90 * time.Second,
		},
		{
			name:      "CI keeps longer timeout unchanged",
			ciValue:   "true",
			requested: 2 * time.Minute,
			want:      2 * time.Minute,
		},
		{
			name:      "zero timeout stays zero in CI",
			ciValue:   "true",
			requested: 0,
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setCIEnv(t, tt.ciValue)
			if got := getCIMinTimeout(tt.requested); got != tt.want {
				t.Fatalf("getCIMinTimeout(%v) = %v, want %v", tt.requested, got, tt.want)
			}
		})
	}
}

func setCIEnv(t *testing.T, value string) {
	t.Helper()
	const key = "CI"

	original, had := os.LookupEnv(key)
	if value == "" {
		_ = os.Unsetenv(key)
	} else {
		_ = os.Setenv(key, value)
	}

	t.Cleanup(func() {
		if !had {
			_ = os.Unsetenv(key)
			return
		}
		_ = os.Setenv(key, original)
	})
}
