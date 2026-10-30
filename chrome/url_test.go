package chrome

import (
	"github.com/koizuka/scraper"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestChromeSession_GetCurrentURL(t *testing.T) {
	t.Run("GetCurrentURL for Session", func(t *testing.T) {
		// Create a test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body><h1>Test Page</h1></body></html>"))
		}))
		defer server.Close()

		// Create session and Chrome session
		session := scraper.NewSession("test", scraper.ConsoleLogger{})
		chromeSession, cancel, err := NewWithRetry(session, newIsolatedTestOptions(t, true, 10*time.Second), 2)
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
		session := scraper.NewSession("test", scraper.ConsoleLogger{})
		chromeSession, cancel, err := NewWithRetry(session, newIsolatedTestOptions(t, true, 10*time.Second), 2)
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

func TestChromeSession_GetCurrentURL_Replay(t *testing.T) {
	t.Run("Session replay mode", func(t *testing.T) {
		// Create test server
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("<html><body><h1>Chrome Test Page</h1></body></html>"))
		}))
		defer server.Close()

		// First, record mode
		session := scraper.NewSession("test_chrome_replay", scraper.ConsoleLogger{})
		chromeSession, cancel, err := NewWithRetry(session, newIsolatedTestOptions(t, true, 10*time.Second), 2)
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
		session.InvokeCount = 1 // Set to match the saved file number

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
