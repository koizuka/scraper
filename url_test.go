package scraper

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
		session.InvokeCount = 0 // Reset counter for replay

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
