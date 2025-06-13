package scraper

import (
	"github.com/chromedp/chromedp"
	"os"
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
		Timeout:           timeout,
		ExtraAllocOptions: getCICompatibleChromeOptions(),
	}
}
