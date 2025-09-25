package scraper

import (
	"context"
	"errors"
	"github.com/PuerkitoBio/goquery"
)

// UnifiedUnmarshal provides a unified interface for data extraction that works
// with both traditional HTTP-based scraping and Chrome-based browser automation.
// It automatically detects the scraper type and uses the appropriate unmarshal function.
func UnifiedUnmarshal(scraper UnifiedScraper, v interface{}, selector string, opt UnmarshalOption) error {
	return scraper.ExtractData(v, selector, opt)
}

// UnifiedUnmarshalWithContext allows specifying a context for Chrome-based unmarshal operations.
// For HTTP-based scraping, the context is ignored.
func UnifiedUnmarshalWithContext(ctx context.Context, scraper UnifiedScraper, v interface{}, selector string, opt UnmarshalOption) error {
	switch scraper.(type) {
	case *ChromeSession:
		// Use Chrome-specific unmarshal with context
		return ChromeUnmarshal(ctx, v, selector, opt)
	case *Session:
		// Use traditional unmarshal - context is ignored
		return scraper.ExtractData(v, selector, opt)
	default:
		return errors.New("unsupported scraper type")
	}
}

// UnifiedUnmarshalFromSelection provides a unified interface for unmarshaling from
// a goquery.Selection. This is primarily for HTTP-based scraping but can be useful
// when you already have HTML content extracted from Chrome.
func UnifiedUnmarshalFromSelection(selection *goquery.Selection, v interface{}, opt UnmarshalOption) error {
	return Unmarshal(v, selection, opt)
}

// ExtractDataFromURL is a convenience function that combines navigation and data extraction
// in a single call, working with both scraper types.
func ExtractDataFromURL(scraper UnifiedScraper, url string, selector string, v interface{}, opt UnmarshalOption) error {
	return scraper.Run(
		Navigate(url),
		WaitVisible(selector),
		ExtractData(v, selector, opt),
	)
}
