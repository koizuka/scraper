package scraper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"
)

// TestThreadSafetyFormData tests concurrent access to pendingFormData
func TestThreadSafetyFormData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body><form><input name="test" type="text"></form></body></html>`))
	}))
	defer server.Close()

	var logger ConsoleLogger
	session := NewSession("test-thread-safety", logger)
	tmpDir := t.TempDir()
	session.FilePrefix = tmpDir + "/"

	// Navigate once
	_, err := session.GetPage(server.URL)
	if err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}

	// Test concurrent SendKeys operations
	const numGoroutines = 50
	const numOperations = 10

	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*numOperations)

	// Launch multiple goroutines doing SendKeys concurrently
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				err := session.SendKeys("input[name=test]", fmt.Sprintf("value-%d-%d", id, j))
				if err != nil {
					errors <- err
					return
				}
				// Small delay to increase chance of race conditions
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent SendKeys error: %v", err)
	}
}

// TestThreadSafetyCurrentPage tests concurrent access to currentPage
func TestThreadSafetyCurrentPage(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`<html><body><h1>Test Page</h1><a href="/page2">Link</a></body></html>`))
	}))
	defer server.Close()

	var logger ConsoleLogger
	session := NewSession("test-current-page", logger)
	tmpDir := t.TempDir()
	session.FilePrefix = tmpDir + "/"

	// Navigate once
	_, err := session.GetPage(server.URL)
	if err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}

	// Test concurrent access to currentPage through different operations
	const numGoroutines = 20
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines*3)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			// Try SavePage (reads currentPage)
			_, err := session.SavePage()
			if err != nil {
				errors <- err
			}

			// Try ExtractData (reads currentPage)
			var data struct {
				Title string `find:"h1"`
			}
			err = session.ExtractData(&data, "body", UnmarshalOption{})
			if err != nil {
				errors <- err
			}

			// Try WaitVisible (reads currentPage)
			err = session.WaitVisible("h1")
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// Check for any errors
	for err := range errors {
		t.Errorf("Concurrent currentPage access error: %v", err)
	}
}

// TestMemoryLeakPrevention tests that pendingFormData is properly cleaned up
func TestMemoryLeakPrevention(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body><form name="test" action="/submit" method="post">
				<input name="username" type="text">
				<input name="password" type="password">
				<input type="submit" value="Submit">
			</form></body></html>`))
		case "/submit":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body><h1>Success</h1></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var logger ConsoleLogger
	session := NewSession("test-memory-leak", logger)
	tmpDir := t.TempDir()
	session.FilePrefix = tmpDir + "/"

	// Navigate to the form page
	_, err := session.GetPage(server.URL)
	if err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}

	// Add form data
	err = session.SendKeys("input[name=username]", "testuser")
	if err != nil {
		t.Errorf("SendKeys error = %v", err)
	}

	err = session.SendKeys("input[name=password]", "testpass")
	if err != nil {
		t.Errorf("SendKeys error = %v", err)
	}

	// Check that pendingFormData has data
	session.mu.RLock()
	dataCount := len(session.pendingFormData)
	session.mu.RUnlock()

	if dataCount == 0 {
		t.Error("Expected pendingFormData to have entries before form submission")
	}

	// Submit form - this should clean up pendingFormData
	err = session.SubmitForm("form[name=test]", nil)
	if err != nil {
		t.Errorf("SubmitForm error = %v", err)
	}

	// Check that pendingFormData is cleaned up
	session.mu.RLock()
	finalDataCount := len(session.pendingFormData)
	session.mu.RUnlock()

	if finalDataCount != 0 {
		t.Errorf("Expected pendingFormData to be empty after form submission, got %d entries", finalDataCount)
	}
}

// TestConcurrentFormSubmissions tests multiple concurrent form submissions
func TestConcurrentFormSubmissions(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body><form name="test" action="/submit" method="post">
				<input name="data" type="text">
				<input type="submit" value="Submit">
			</form></body></html>`))
		case "/submit":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<html><body><h1>Success</h1></body></html>`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var logger ConsoleLogger
	session := NewSession("test-concurrent-forms", logger)
	tmpDir := t.TempDir()
	session.FilePrefix = tmpDir + "/"

	// Navigate to the form page
	_, err := session.GetPage(server.URL)
	if err != nil {
		t.Fatalf("GetPage() error = %v", err)
	}

	// Test concurrent form submissions
	const numSubmissions = 10
	var wg sync.WaitGroup
	errors := make(chan error, numSubmissions)

	for i := 0; i < numSubmissions; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			formData := map[string]string{
				"data": fmt.Sprintf("submission-%d", id),
			}

			err := session.SubmitForm("form[name=test]", formData)
			if err != nil {
				errors <- err
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	// We expect some errors due to concurrent access, but no panics or race conditions
	errorCount := 0
	for err := range errors {
		errorCount++
		t.Logf("Concurrent form submission error (expected): %v", err)
	}

	t.Logf("Got %d errors out of %d concurrent form submissions", errorCount, numSubmissions)
}
