package scraper

import (
	"context"
	"github.com/chromedp/chromedp"
	"os"
	"testing"
	"time"
)

// getCICompatibleChromeOptions returns Chrome allocator options that work in CI environments.
// This function is intended for use in tests only.
func getCICompatibleChromeOptions() []chromedp.ExecAllocatorOption {
	options := []chromedp.ExecAllocatorOption{}

	// Add CI-specific options when running in CI environment
	if os.Getenv("CI") == "true" {
		options = append(options,
			chromedp.NoSandbox,
			chromedp.NoFirstRun,
			chromedp.Flag("disable-dev-shm-usage", true),
			chromedp.Flag("disable-extensions", true),
			chromedp.Flag("disable-default-apps", true),
			chromedp.Flag("disable-web-security", true),
			chromedp.Flag("disable-blink-features", "AutomationControlled"),
			chromedp.Flag("single-process", true),
			// Increase WebSocket URL read timeout for Chrome startup in CI
			chromedp.WSURLReadTimeout(60*time.Second),
		)
	}

	return options
}

// NewTestChromeOptions creates Chrome options with CI compatibility built-in.
// This helper function simplifies test code by automatically including
// CI-compatible options when needed.
func NewTestChromeOptions(headless bool) NewChromeOptions {
	return NewChromeOptions{
		Headless:          headless,
		ExtraAllocOptions: getCICompatibleChromeOptions(),
	}
}

// NewTestChromeOptionsWithTimeout creates Chrome options with CI compatibility and custom timeout.
// This helper function simplifies test code by automatically including
// CI-compatible options when needed, with a custom timeout setting.
func NewTestChromeOptionsWithTimeout(headless bool, timeout time.Duration) NewChromeOptions {
	return NewChromeOptions{
		Headless:          headless,
		Timeout:           getCIMinTimeout(timeout),
		ExtraAllocOptions: getCICompatibleChromeOptions(),
	}
}

// getCIMinTimeout ensures that CI environments have ample time to launch Chrome,
// which is slower on shared runners. Local runs retain their requested timeout.
// A zero duration still means “no timeout” even in CI, so we skip adjustments.
func getCIMinTimeout(requested time.Duration) time.Duration {
	if requested == 0 || os.Getenv("CI") != "true" {
		return requested
	}

	const minCITimeout = 90 * time.Second
	if requested < minCITimeout {
		return minCITimeout
	}
	return requested
}

// NewTestChromeContext creates an isolated Chrome browser context for tests.
// It uses t.TempDir() for the user data directory to avoid conflicts with other
// Chrome instances, and applies CI-aware timeouts.
// Retries up to 2 times on Chrome startup failure (flaky CI environments).
// Cleanup is registered via t.Cleanup, so callers don't need to defer cancel.
func NewTestChromeContext(t *testing.T, timeout time.Duration) context.Context {
	t.Helper()

	const maxRetries = 2

	testOptions := NewTestChromeOptions(true)
	effectiveTimeout := getCIMinTimeout(timeout)
	if effectiveTimeout == 0 {
		effectiveTimeout = 30 * time.Second
	}

	var ctx context.Context
	var timeoutCancel, browserCancel, allocCancel context.CancelFunc

	for attempt := 0; attempt <= maxRetries; attempt++ {
		allocOptions := []chromedp.ExecAllocatorOption{
			chromedp.UserDataDir(t.TempDir()),
		}
		if testOptions.Headless {
			allocOptions = append(allocOptions, chromedp.Headless, chromedp.DisableGPU)
		}
		allocOptions = append(allocOptions, testOptions.ExtraAllocOptions...)

		var allocCtx, browserCtx context.Context
		allocCtx, allocCancel = chromedp.NewExecAllocator(context.Background(), allocOptions...)
		browserCtx, browserCancel = chromedp.NewContext(allocCtx)
		ctx, timeoutCancel = context.WithTimeout(browserCtx, effectiveTimeout)

		// Try to start Chrome by running a no-op
		if err := chromedp.Run(ctx); err != nil {
			timeoutCancel()
			browserCancel()
			allocCancel()
			if attempt < maxRetries {
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
			t.Fatalf("Chrome failed to start after %d retries: %v", maxRetries+1, err)
		}
		break
	}

	t.Cleanup(func() {
		timeoutCancel()
		browserCancel()
		allocCancel()
	})

	return ctx
}

// NewChromeWithRetry creates a new Chrome session with retry logic for startup failures.
// This helper is specifically designed to handle flaky Chrome startup issues in CI environments.
// It retries on any Chrome startup failure (crashes, sandbox issues, websocket timeouts, etc.).
func NewChromeWithRetry(session *Session, options NewChromeOptions, maxRetries int) (*ChromeSession, func(), error) {
	var chromeSession *ChromeSession
	var cancelFunc func()
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		chromeSession, cancelFunc, err = session.NewChromeOpt(options)
		if err == nil {
			return chromeSession, cancelFunc, nil
		}

		if cancelFunc != nil {
			cancelFunc()
		}
		if attempt < maxRetries {
			// Wait a bit before retrying
			time.Sleep(time.Duration(attempt+1) * time.Second)
			continue
		}
	}

	return nil, func() {}, err
}
