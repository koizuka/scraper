package scraper

import (
	"time"
)

const (
	// DefaultTimeout is the default timeout for navigation and element waiting
	DefaultTimeout = 30 * time.Second
	// DefaultDownloadTimeout is the default timeout for file downloads
	DefaultDownloadTimeout = 60 * time.Second
	// DefaultFormTimeout is the default timeout for form operations
	DefaultFormTimeout = 15 * time.Second
)

// UnifiedScraper provides a common interface for both traditional HTTP-based scraping
// and Chrome-based browser automation, allowing code to work seamlessly with either backend.
//
// Example usage:
//
//	var scraper UnifiedScraper = session // or chromeSession
//	err := scraper.DoNavigate("https://example.com")
//	err = scraper.DoWaitVisible(".content")
//	data, err := scraper.SavePage()
//
// The interface abstracts the differences between HTTP and browser-based scraping:
// - HTTP scraping: DoWaitVisible is a no-op, form operations are simulated
// - Chrome scraping: Full browser automation with real user interactions
type UnifiedScraper interface {
	// Navigation methods
	DoNavigate(url string) error
	DoWaitVisible(selector string) error

	// Form interaction methods
	DoSendKeys(selector, value string) error
	DoClick(selector string) error
	SubmitForm(formSelector string, params map[string]string) error
	FollowAnchor(text string) error

	// Data extraction methods
	SavePage() (string, error)
	ExtractData(v interface{}, selector string, opt UnmarshalOption) error

	// Download methods
	DownloadResource(options UnifiedDownloadOptions) (string, error)

	// Debug methods
	GetDebugStep() string
	SetDebugStep(step string)
	ClearDebugStep()

	// Utility methods
	Printf(format string, a ...interface{})
}

// UnifiedDownloadOptions provides options for file downloads that work
// with both HTTP-based and browser-based download mechanisms.
type UnifiedDownloadOptions struct {
	Timeout time.Duration // Maximum time to wait for download (defaults to DefaultDownloadTimeout if zero)
	Glob    string        // File name pattern to match (for Chrome downloads)
	SaveAs  string        // Target filename (optional)
}

// ScraperType indicates which underlying scraping mechanism is being used
type ScraperType int

const (
	HTTPScraper ScraperType = iota
	ChromeScraper
)

// GetScraperType returns the type of scraper being used
func GetScraperType(scraper UnifiedScraper) ScraperType {
	switch scraper.(type) {
	case *Session:
		return HTTPScraper
	case *ChromeSession:
		return ChromeScraper
	default:
		return HTTPScraper
	}
}
