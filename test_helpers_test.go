package scraper

// Test-only helpers. Unlike test_helpers.go, this file is excluded from the
// public package API because it is only compiled for tests.

import (
	"os"
	"testing"
	"time"
)

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
