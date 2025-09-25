package scraper

import (
	"fmt"
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

// UnifiedAction represents a single scraping operation that can be executed
// on either HTTP or Chrome backends. Actions are composable and support replay mode.
type UnifiedAction interface {
	// Do executes the action on the given scraper
	Do(scraper UnifiedScraper) error
}

// UnifiedScraper provides a common interface for both traditional HTTP-based scraping
// and Chrome-based browser automation, using an action-based approach similar to chromedp.
//
// Example usage:
//
//	var scraper UnifiedScraper = session // or chromeSession
//	err := scraper.Run(
//		Navigate("https://example.com"),
//		WaitVisible(".content"),
//		SendKeys("#user", userId),
//		Click("#submit"),
//		SavePage(),
//	)
//
// The interface abstracts the differences between HTTP and browser-based scraping:
// - HTTP scraping: WaitVisible is a no-op, form operations are simulated
// - Chrome scraping: Full browser automation with real user interactions
type UnifiedScraper interface {
	// Core execution method - runs a sequence of actions
	Run(actions ...UnifiedAction) error

	// State query methods
	GetCurrentURL() (string, error)
	IsReplayMode() bool

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

// ActionFunc allows custom actions to be created from functions
type ActionFunc func(scraper UnifiedScraper) error

// Do implements UnifiedAction.Do
func (fn ActionFunc) Do(scraper UnifiedScraper) error {
	return fn(scraper)
}

// ScraperType indicates which underlying scraping mechanism is being used
type ScraperType int

const (
	HTTPScraper ScraperType = iota
	ChromeScraper
)

// Core action constructors - these return UnifiedAction instances

// Navigate creates an action that navigates to the specified URL
func Navigate(url string) UnifiedAction {
	return ActionFunc(func(scraper UnifiedScraper) error {
		if chromeSession, ok := scraper.(*ChromeSession); ok {
			return chromeSession.navigateAction(url)
		} else if httpSession, ok := scraper.(*Session); ok {
			return httpSession.navigateAction(url)
		}
		return fmt.Errorf("unsupported scraper type")
	})
}

// WaitVisible creates an action that waits for an element to be visible
func WaitVisible(selector string) UnifiedAction {
	return ActionFunc(func(scraper UnifiedScraper) error {
		if chromeSession, ok := scraper.(*ChromeSession); ok {
			return chromeSession.waitVisibleAction(selector)
		} else if httpSession, ok := scraper.(*Session); ok {
			return httpSession.waitVisibleAction(selector)
		}
		return fmt.Errorf("unsupported scraper type")
	})
}

// SendKeys creates an action that sends keys to an element
func SendKeys(selector, value string) UnifiedAction {
	return ActionFunc(func(scraper UnifiedScraper) error {
		if chromeSession, ok := scraper.(*ChromeSession); ok {
			return chromeSession.sendKeysAction(selector, value)
		} else if httpSession, ok := scraper.(*Session); ok {
			return httpSession.sendKeysAction(selector, value)
		}
		return fmt.Errorf("unsupported scraper type")
	})
}

// Click creates an action that clicks an element
func Click(selector string) UnifiedAction {
	return ActionFunc(func(scraper UnifiedScraper) error {
		if chromeSession, ok := scraper.(*ChromeSession); ok {
			return chromeSession.clickAction(selector)
		} else if httpSession, ok := scraper.(*Session); ok {
			return httpSession.clickAction(selector)
		}
		return fmt.Errorf("unsupported scraper type")
	})
}

// Sleep creates an action that pauses execution for the specified duration
func Sleep(duration time.Duration) UnifiedAction {
	return ActionFunc(func(scraper UnifiedScraper) error {
		if scraper.IsReplayMode() {
			// In replay mode, skip sleep and just log
			scraper.Printf("REPLAY SLEEP: skipping %v", duration)
			return nil
		}
		if chromeSession, ok := scraper.(*ChromeSession); ok {
			return chromeSession.sleepAction(duration)
		} else if httpSession, ok := scraper.(*Session); ok {
			return httpSession.sleepAction(duration)
		}
		return fmt.Errorf("unsupported scraper type")
	})
}

// SavePage creates an action that saves the current page HTML
func SavePage() UnifiedAction {
	return ActionFunc(func(scraper UnifiedScraper) error {
		if chromeSession, ok := scraper.(*ChromeSession); ok {
			return chromeSession.savePageAction()
		} else if httpSession, ok := scraper.(*Session); ok {
			return httpSession.savePageAction()
		}
		return fmt.Errorf("unsupported scraper type")
	})
}

// ExtractData creates an action that extracts data using CSS selectors
func ExtractData(v interface{}, selector string, opt UnmarshalOption) UnifiedAction {
	return ActionFunc(func(scraper UnifiedScraper) error {
		if chromeSession, ok := scraper.(*ChromeSession); ok {
			return chromeSession.extractDataAction(v, selector, opt)
		} else if httpSession, ok := scraper.(*Session); ok {
			return httpSession.extractDataAction(v, selector, opt)
		}
		return fmt.Errorf("unsupported scraper type")
	})
}

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
