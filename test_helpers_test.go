package scraper

// Test-only helpers. Unlike test_helpers.go, this file is excluded from the
// public package API because it is only compiled for tests.

import (
	"testing"
	"time"
)

// newIsolatedTestChromeOptions creates CI-compatible Chrome options that use a
// per-test user data directory (t.TempDir()) instead of the shared
// ./chromeUserData. Consecutive tests sharing one profile directory can make
// Chrome instances interfere with each other (profile SingletonLock contention,
// leftover state), which shows up as flaky "context canceled" failures in CI.
func newIsolatedTestChromeOptions(t *testing.T, headless bool, timeout time.Duration) NewChromeOptions {
	t.Helper()
	options := NewTestChromeOptionsWithTimeout(headless, timeout)
	options.UserDataDir = t.TempDir()
	return options
}
