package scraper

// Tests for test_helpers.go, plus test-only helpers. Unlike test_helpers.go,
// helpers in this file are excluded from the public package API because it is
// only compiled for tests.

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

// newIsolatedTestChromeOptions creates CI-compatible Chrome options that use a
// per-test user data directory instead of the shared ./chromeUserData.
// Consecutive tests sharing one profile directory can make Chrome instances
// interfere with each other (profile SingletonLock contention, leftover
// state), which shows up as flaky "context canceled" failures in CI.
//
// The directory is deliberately NOT t.TempDir(): its cleanup fails the test
// when files are still locked, and on Windows Chrome's crashpad handler (a
// separate process the allocator does not wait for) can briefly keep e.g.
// CrashpadMetrics-active.pma open after the browser exits. Cleanup is
// best-effort instead.
func newIsolatedTestChromeOptions(t *testing.T, headless bool, timeout time.Duration) NewChromeOptions {
	t.Helper()
	options := NewTestChromeOptionsWithTimeout(headless, timeout)
	dir, err := os.MkdirTemp("", "chromeUserData-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	options.UserDataDir = dir
	return options
}
