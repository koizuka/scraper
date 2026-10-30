package chrome

import (
	"testing"

	"github.com/koizuka/scraper"
)

var (
	_ scraper.UnifiedScraper   = (*Session)(nil)
	_ scraper.ScraperTyper     = (*Session)(nil)
	_ scraper.ContextExtractor = (*Session)(nil)
)

func TestScraperTypeDetection(t *testing.T) {
	chromeSession := &Session{Session: scraper.NewSession("test-type", scraper.ConsoleLogger{})}
	if got := scraper.GetScraperType(chromeSession); got != scraper.ChromeScraper {
		t.Errorf("GetScraperType(chromeSession) = %v, want %v", got, scraper.ChromeScraper)
	}
}
