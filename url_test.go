package scraper

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSession_GetCurrentURL(t *testing.T) {
	t.Run("GetCurrentURL for Session", func(t *testing.T) {
		// Create a test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body><h1>Test Page</h1></body></html>"))
		}))
		defer server.Close()

		// Create session and get a page
		session := NewSession("test", ConsoleLogger{})
		page, err := session.GetPage(server.URL)
		if err != nil {
			t.Fatalf("Failed to get page: %v", err)
		}

		// Test GetCurrentURL
		currentURL, err := session.GetCurrentURL()
		if err != nil {
			t.Fatalf("Failed to get current URL: %v", err)
		}

		if currentURL != server.URL {
			t.Errorf("Expected URL %s, got %s", server.URL, currentURL)
		}

		// Verify that the returned URL matches the page's base URL
		if page.BaseUrl.String() != currentURL {
			t.Errorf("Current URL doesn't match page base URL: %s != %s", page.BaseUrl.String(), currentURL)
		}
	})

	t.Run("GetCurrentURL without page", func(t *testing.T) {
		// Create session without getting any page
		session := NewSession("test", ConsoleLogger{})

		// Test GetCurrentURL should return error
		_, err := session.GetCurrentURL()
		if err == nil {
			t.Error("Expected error when no current page available")
		}
	})
}

func TestChromeSession_GetCurrentURL(t *testing.T) {
	t.Run("GetCurrentURL for ChromeSession", func(t *testing.T) {
		// Create a test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body><h1>Test Page</h1></body></html>"))
		}))
		defer server.Close()

		// Create session and Chrome session
		session := NewSession("test", ConsoleLogger{})
		chromeSession, cancel, err := session.NewChromeOpt(NewChromeOptions{
			Headless: true,
			Timeout:  10 * time.Second,
		})
		if err != nil {
			t.Skipf("Chrome not available for testing: %v", err)
		}
		defer cancel()

		// Navigate to test page
		err = chromeSession.DoNavigate(server.URL)
		if err != nil {
			t.Fatalf("Failed to navigate: %v", err)
		}

		// Test GetCurrentURL
		currentURL, err := chromeSession.GetCurrentURL()
		if err != nil {
			t.Fatalf("Failed to get current URL: %v", err)
		}

		// Allow for trailing slash differences
		expectedURL := server.URL
		if currentURL != expectedURL && currentURL != expectedURL+"/" {
			t.Errorf("Expected URL %s or %s/, got %s", expectedURL, expectedURL, currentURL)
		}
	})

	t.Run("GetCurrentURL without navigation", func(t *testing.T) {
		// Create session and Chrome session
		session := NewSession("test", ConsoleLogger{})
		chromeSession, cancel, err := session.NewChromeOpt(NewChromeOptions{
			Headless: true,
			Timeout:  10 * time.Second,
		})
		if err != nil {
			t.Skipf("Chrome not available for testing: %v", err)
		}
		defer cancel()

		// Test GetCurrentURL without navigation - should get about:blank or empty
		currentURL, err := chromeSession.GetCurrentURL()
		if err != nil {
			t.Fatalf("Failed to get current URL: %v", err)
		}

		// Chrome typically starts with about:blank
		if currentURL != "about:blank" && currentURL != "" {
			t.Logf("Initial Chrome URL: %s", currentURL)
		}
	})
}

func TestSession_GetCurrentURL_Replay(t *testing.T) {
	t.Run("Session replay mode", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body><h1>Test Page</h1></body></html>"))
		}))
		defer server.Close()

		// First, record mode
		session := NewSession("test_replay", ConsoleLogger{})
		session.SaveToFile = true
		page, err := session.GetPage(server.URL)
		if err != nil {
			t.Fatalf("Failed to get page in record mode: %v", err)
		}

		expectedURL := page.BaseUrl.String()

		// Now replay mode
		session.NotUseNetwork = true
		session.invokeCount = 0 // Reset counter for replay

		replayPage, err := session.GetPage(server.URL) // URL doesn't matter in replay mode
		if err != nil {
			t.Fatalf("Failed to get page in replay mode: %v", err)
		}

		// Test GetCurrentURL in replay mode
		currentURL, err := session.GetCurrentURL()
		if err != nil {
			t.Fatalf("Failed to get current URL in replay mode: %v", err)
		}

		if currentURL != expectedURL {
			t.Errorf("Replay URL mismatch: expected %s, got %s", expectedURL, currentURL)
		}

		// Verify that the replayed page has correct URL
		if replayPage.BaseUrl.String() != expectedURL {
			t.Errorf("Replayed page URL mismatch: expected %s, got %s", expectedURL, replayPage.BaseUrl.String())
		}
	})
}

func TestChromeSession_GetCurrentURL_Replay(t *testing.T) {
	t.Run("ChromeSession replay mode", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body><h1>Chrome Test Page</h1></body></html>"))
		}))
		defer server.Close()

		// First, record mode
		session := NewSession("test_chrome_replay", ConsoleLogger{})
		chromeSession, cancel, err := session.NewChromeOpt(NewChromeOptions{
			Headless: true,
			Timeout:  10 * time.Second,
		})
		if err != nil {
			t.Skipf("Chrome not available for testing: %v", err)
		}
		defer cancel()

		// Navigate and save
		err = chromeSession.DoNavigate(server.URL)
		if err != nil {
			t.Fatalf("Failed to navigate in record mode: %v", err)
		}

		expectedURL, err := chromeSession.GetCurrentURL()
		if err != nil {
			t.Fatalf("Failed to get URL in record mode: %v", err)
		}

		// Now test replay mode
		session.NotUseNetwork = true
		session.invokeCount = 1 // Set to match the saved file number

		// In replay mode, GetCurrentURL should return saved URL
		replayURL, err := chromeSession.GetCurrentURL()
		if err != nil {
			t.Fatalf("Failed to get current URL in replay mode: %v", err)
		}

		// Allow for trailing slash differences
		if replayURL != expectedURL && replayURL != expectedURL+"/" && expectedURL != replayURL+"/" {
			t.Errorf("Replay URL mismatch: expected %s, got %s", expectedURL, replayURL)
		}
	})
}
