package scraper

import (
	"github.com/chromedp/chromedp"
	"os"
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