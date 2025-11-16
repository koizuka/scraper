package scraper

import (
	"github.com/chromedp/chromedp"
	"os"
	"strings"
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

// NewChromeWithRetry creates a new Chrome session with retry logic for startup failures.
// This helper is specifically designed to handle flaky Chrome startup issues in CI environments.
func NewChromeWithRetry(session *Session, options NewChromeOptions, maxRetries int) (*ChromeSession, func(), error) {
	var chromeSession *ChromeSession
	var cancelFunc func()
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		chromeSession, cancelFunc, err = session.NewChromeOpt(options)
		if err == nil {
			return chromeSession, cancelFunc, nil
		}

		// Check if it's a websocket timeout error that we should retry
		if strings.Contains(err.Error(), "websocket url timeout reached") {
			if cancelFunc != nil {
				cancelFunc()
			}
			if attempt < maxRetries {
				// Wait a bit before retrying
				time.Sleep(time.Duration(attempt+1) * time.Second)
				continue
			}
		}

		// For other errors or if we've exhausted retries, return the error
		if cancelFunc != nil {
			cancelFunc()
		}
		return nil, func() {}, err
	}

	return chromeSession, cancelFunc, err
}
