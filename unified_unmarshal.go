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

// ContextExtractor is implemented by scrapers whose data extraction depends on
// a context (e.g. the Chrome session in the github.com/koizuka/scraper/chrome module).
type ContextExtractor interface {
	ExtractDataWithContext(ctx context.Context, v interface{}, selector string, opt UnmarshalOption) error
}

// UnifiedUnmarshalWithContext allows specifying a context for Chrome-based unmarshal operations.
// For HTTP-based scraping, the context is ignored.
func UnifiedUnmarshalWithContext(ctx context.Context, scraper UnifiedScraper, v interface{}, selector string, opt UnmarshalOption) error {
	switch s := scraper.(type) {
	case ContextExtractor:
		// Use scraper-specific unmarshal with context (e.g. Chrome)
		return s.ExtractDataWithContext(ctx, v, selector, opt)
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

// ExtractData is a convenience function that combines navigation and data extraction
// in a single call, working with both scraper types.
func ExtractData(scraper UnifiedScraper, url string, selector string, v interface{}, opt UnmarshalOption) error {
	err := scraper.DoNavigate(url)
	if err != nil {
		return err
	}

	err = scraper.DoWaitVisible(selector)
	if err != nil {
		return err
	}

	return scraper.ExtractData(v, selector, opt)
}
