// Package chrome provides Chrome/Chromium browser automation for
// github.com/koizuka/scraper using chromedp.
//
// It is a separate module so that programs using only the HTTP-based
// scraper do not depend on chromedp.
//
//	session := scraper.NewSession("session-name", logger)
//	chromeSession, cancel, err := chrome.New(session, chrome.Options{Headless: true})
//	defer cancel()
//	resp, err := chromeSession.RunNavigate("https://example.com")
package chrome
