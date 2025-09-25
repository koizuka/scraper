package scraper

import (
	"strings"
	"testing"
	"time"
)

// TestUnifiedActionBasics tests the basic functionality of the new Action-based API
func TestUnifiedActionBasics(t *testing.T) {
	logger := &TestLogger{}
	session := NewSession("unified-action-test", logger)

	// Test basic actions with HTTP scraper
	testBasicActions(t, session, "HTTP Session")

	// Test with Chrome scraper if available (skip if Chrome not available)
	if testing.Short() {
		t.Skip("Skipping Chrome tests in short mode")
	}

	chromeSession, cancel, err := session.NewChrome()
	if err != nil {
		t.Skipf("Chrome not available, skipping Chrome tests: %v", err)
	}
	defer cancel()

	testBasicActions(t, chromeSession, "Chrome Session")
}

func testBasicActions(t *testing.T, scraper UnifiedScraper, scraperName string) {
	t.Run(scraperName, func(t *testing.T) {
		t.Run("Run with single action", func(t *testing.T) {
			// Test single action execution
			err := scraper.Run(
				Navigate("https://httpbin.org/html"),
			)
			if err != nil {
				t.Fatalf("Single action failed: %v", err)
			}
		})

		t.Run("Run with multiple actions", func(t *testing.T) {
			// Test multiple actions
			err := scraper.Run(
				Navigate("https://httpbin.org/html"),
				WaitVisible("body"),
				SavePage(),
			)
			if err != nil {
				t.Fatalf("Multiple actions failed: %v", err)
			}
		})

		t.Run("GetCurrentURL", func(t *testing.T) {
			err := scraper.Run(
				Navigate("https://httpbin.org/html"),
			)
			if err != nil {
				t.Fatalf("Navigate failed: %v", err)
			}

			url, err := scraper.GetCurrentURL()
			if err != nil {
				t.Fatalf("GetCurrentURL failed: %v", err)
			}

			if !strings.Contains(url, "httpbin.org") {
				t.Errorf("Expected URL to contain 'httpbin.org', got: %s", url)
			}
		})

		t.Run("IsReplayMode", func(t *testing.T) {
			// In these tests, we should not be in replay mode initially
			if scraper.IsReplayMode() {
				t.Error("Expected IsReplayMode to be false in live mode")
			}
		})

		t.Run("Sleep action", func(t *testing.T) {
			start := time.Now()
			err := scraper.Run(
				Sleep(100 * time.Millisecond),
			)
			elapsed := time.Since(start)

			if err != nil {
				t.Fatalf("Sleep action failed: %v", err)
			}

			// In live mode, sleep should actually wait
			if !scraper.IsReplayMode() && elapsed < 50*time.Millisecond {
				t.Errorf("Sleep didn't wait long enough: %v", elapsed)
			}
		})
	})
}

// TestCustomActions tests creating and using custom actions
func TestCustomActions(t *testing.T) {
	logger := &TestLogger{}
	session := NewSession("custom-action-test", logger)

	// Custom action that combines multiple basic actions
	customNavigateAndSave := ActionFunc(func(scraper UnifiedScraper) error {
		return scraper.Run(
			Navigate("https://httpbin.org/html"),
			WaitVisible("body"),
			SavePage(),
		)
	})

	// Test custom action
	err := session.Run(customNavigateAndSave)
	if err != nil {
		t.Fatalf("Custom action failed: %v", err)
	}

	// Verify the action worked
	url, err := session.GetCurrentURL()
	if err != nil {
		t.Fatalf("GetCurrentURL after custom action failed: %v", err)
	}

	if !strings.Contains(url, "httpbin.org") {
		t.Errorf("Custom action didn't navigate correctly, URL: %s", url)
	}
}

// TestConditionalActions tests conditional logic with actions
func TestConditionalActions(t *testing.T) {
	logger := &TestLogger{}
	session := NewSession("conditional-action-test", logger)

	// Conditional action based on URL
	conditionalAction := ActionFunc(func(scraper UnifiedScraper) error {
		// First navigate
		err := scraper.Run(Navigate("https://httpbin.org/html"))
		if err != nil {
			return err
		}

		// Check URL and perform different actions based on it
		url, err := scraper.GetCurrentURL()
		if err != nil {
			return err
		}

		if strings.Contains(url, "httpbin.org") {
			// If it's httpbin, save page
			return scraper.Run(SavePage())
		} else {
			// Otherwise, just wait
			return scraper.Run(Sleep(100 * time.Millisecond))
		}
	})

	err := session.Run(conditionalAction)
	if err != nil {
		t.Fatalf("Conditional action failed: %v", err)
	}
}

// TestDataExtraction tests data extraction with the new API
func TestDataExtraction(t *testing.T) {
	logger := &TestLogger{}
	session := NewSession("data-extraction-test", logger)

	// Navigate to a page with some structure
	err := session.Run(
		Navigate("https://httpbin.org/html"),
		WaitVisible("body"),
	)
	if err != nil {
		t.Fatalf("Navigation failed: %v", err)
	}

	// Extract data
	var data struct {
		Title string `find:"title"`
	}

	err = session.Run(
		ExtractData(&data, "html", UnmarshalOption{}),
	)
	if err != nil {
		t.Fatalf("Data extraction failed: %v", err)
	}

	if data.Title == "" {
		t.Error("Expected to extract title, but got empty string")
	}
}

// TestActionChaining tests complex action chaining
func TestActionChaining(t *testing.T) {
	logger := &TestLogger{}
	session := NewSession("action-chaining-test", logger)

	// Create a complex chain of actions
	err := session.Run(
		Navigate("https://httpbin.org/html"),
		WaitVisible("body"),
		SavePage(),
		Sleep(50*time.Millisecond),
		// Custom inline action
		ActionFunc(func(scraper UnifiedScraper) error {
			url, err := scraper.GetCurrentURL()
			if err != nil {
				return err
			}
			scraper.Printf("Current URL in chain: %s", url)
			return nil
		}),
	)

	if err != nil {
		t.Fatalf("Action chaining failed: %v", err)
	}
}

// TestErrorHandling tests error handling in action chains
func TestErrorHandling(t *testing.T) {
	logger := &TestLogger{}
	session := NewSession("error-handling-test", logger)

	// Action that should fail
	failingAction := ActionFunc(func(scraper UnifiedScraper) error {
		return scraper.Run(
			Navigate("https://invalid-url-that-should-fail.example"),
		)
	})

	// This should return an error and stop the chain
	err := session.Run(
		Navigate("https://httpbin.org/html"), // This should work
		failingAction,                        // This should fail
		SavePage(),                           // This should not execute
	)

	if err == nil {
		t.Error("Expected error from failing action, but got nil")
	}
}

// Benchmark the new API vs individual method calls
func BenchmarkUnifiedActions(b *testing.B) {
	logger := &TestLogger{}
	session := NewSession("benchmark-test", logger)

	b.Run("ActionAPI", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			err := session.Run(
				Navigate("https://httpbin.org/html"),
				WaitVisible("body"),
				SavePage(),
			)
			if err != nil {
				b.Fatalf("Action API benchmark failed: %v", err)
			}
		}
	})
}

// TestLogger is a simple logger for testing
type TestLogger struct{}

func (l *TestLogger) Printf(format string, a ...interface{}) {
	// In tests, we might want to be quiet or use testing.T.Logf
}