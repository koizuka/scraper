package scraper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Test HTML content for unified tests
const testHTML = `
<!DOCTYPE html>
<html>
<head>
    <title>Test Page</title>
</head>
<body>
    <h1>Test Header</h1>
    <div class="content">
        <p>Test content</p>
        <a href="/link1" class="test-link">Link 1</a>
        <a href="/link2">Link 2</a>
    </div>
    <form name="test-form" action="/submit" method="post">
        <input type="text" name="username" placeholder="Username">
        <input type="password" name="password" placeholder="Password">
        <input type="submit" value="Submit">
    </form>
    <div class="data-section">
        <div class="item">
            <span class="name">Item 1</span>
            <span class="price">$10.99</span>
        </div>
        <div class="item">
            <span class="name">Item 2</span>
            <span class="price">$25.50</span>
        </div>
    </div>
</body>
</html>`

// TestUnifiedInterface tests the unified scraper interface with both HTTP and Chrome scrapers
func TestUnifiedInterface(t *testing.T) {
	// Create test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testHTML))
		case "/link1":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body><h1>Link 1 Page</h1></body></html>"))
		case "/submit":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body><h1>Form Submitted</h1></body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	testCases := []struct {
		name    string
		scraper func(t *testing.T) UnifiedScraper
		cleanup func()
	}{
		{
			name: "HTTP Scraper",
			scraper: func(t *testing.T) UnifiedScraper {
				var logger ConsoleLogger
				session := NewSession("test-http", logger)
				// Set up temporary directory
				tmpDir := t.TempDir()
				session.FilePrefix = tmpDir + "/"
				return session
			},
			cleanup: func() {},
		},
		// Note: Chrome scraper tests would require actual Chrome installation
		// Uncomment the following when Chrome tests are needed
		/*
			{
				name: "Chrome Scraper",
				scraper: func(t *testing.T) UnifiedScraper {
					var logger ConsoleLogger
					session := NewSession("test-chrome", logger)
					chromeSession, cancel, err := session.NewChromeOpt(NewChromeOptions{
						Headless: true,
						Timeout:  30 * time.Second,
					})
					if err != nil {
						t.Skipf("Chrome not available: %v", err)
					}
					t.Cleanup(cancel)
					return chromeSession
				},
				cleanup: func() {},
			},
		*/
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			scraper := tc.scraper(t)
			defer tc.cleanup()

			// Test Navigation
			err := scraper.DoNavigate(server.URL)
			if err != nil {
				t.Errorf("Navigate() error = %v", err)
				return
			}

			// Test WaitVisible (should be no-op for HTTP, actual wait for Chrome)
			err = scraper.DoWaitVisible("h1")
			if err != nil {
				t.Errorf("WaitVisible() error = %v", err)
			}

			// Test SavePage
			filename, err := scraper.SavePage()
			if err != nil {
				t.Errorf("SavePage() error = %v", err)
			}
			if filename == "" {
				t.Errorf("SavePage() returned empty filename")
			}

			// Test data extraction
			type PageData struct {
				Title string   `find:"h1"`
				Links []string `find:"a" attr:"href"`
			}

			var data PageData
			err = scraper.ExtractData(&data, "body", UnmarshalOption{})
			if err != nil {
				t.Errorf("ExtractData() error = %v", err)
			}

			if data.Title != "Test Header" {
				t.Errorf("ExtractData() title = %v, want %v", data.Title, "Test Header")
			}

			if len(data.Links) < 2 {
				t.Errorf("ExtractData() links count = %v, want >= 2", len(data.Links))
			}
		})
	}
}

// TestUnifiedFormOperations tests form operations with unified interface
func TestUnifiedFormOperations(t *testing.T) {
	// Create test server that captures form submissions
	var lastFormData map[string]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/":
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(testHTML))
		case "/submit":
			if r.Method == "POST" {
				r.ParseForm()
				lastFormData = make(map[string]string)
				for key, values := range r.Form {
					if len(values) > 0 {
						lastFormData[key] = values[0]
					}
				}
			}
			w.Header().Set("Content-Type", "text/html")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("<html><body><h1>Form Submitted</h1></body></html>"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	var logger ConsoleLogger
	session := NewSession("test-form", logger)
	// Set up temporary directory
	tmpDir := t.TempDir()
	session.FilePrefix = tmpDir + "/"
	scraper := UnifiedScraper(session)

	// Navigate to test page
	err := scraper.DoNavigate(server.URL)
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	// Test SendKeys
	err = scraper.DoSendKeys("input[name=username]", "testuser")
	if err != nil {
		t.Errorf("SendKeys() error = %v", err)
	}

	err = scraper.DoSendKeys("input[name=password]", "testpass")
	if err != nil {
		t.Errorf("SendKeys() error = %v", err)
	}

	// Test form submission
	err = scraper.SubmitForm("form[name=test-form]", nil)
	if err != nil {
		t.Errorf("SubmitForm() error = %v", err)
	}

	// For HTTP scraper, verify form data was submitted
	if GetScraperType(scraper) == HTTPScraper {
		if lastFormData == nil {
			t.Error("Form data was not submitted")
		} else {
			if lastFormData["username"] != "testuser" {
				t.Errorf("Form username = %v, want %v", lastFormData["username"], "testuser")
			}
			if lastFormData["password"] != "testpass" {
				t.Errorf("Form password = %v, want %v", lastFormData["password"], "testpass")
			}
		}
	}
}

// TestUnifiedErrorHandling tests error handling scenarios
func TestUnifiedErrorHandling(t *testing.T) {
	var logger ConsoleLogger
	session := NewSession("test-error", logger)
	// Set up temporary directory
	tmpDir := t.TempDir()
	session.FilePrefix = tmpDir + "/"
	scraper := UnifiedScraper(session)

	// Test clicking without navigation
	err := scraper.DoClick(".nonexistent")
	if err == nil {
		t.Error("Click() should return error when no page is loaded")
	}
	if !strings.Contains(err.Error(), "no current page") {
		t.Errorf("Click() error = %v, should contain 'no current page'", err)
	}

	// Test form submission without navigation
	err = scraper.SubmitForm("form", map[string]string{"test": "value"})
	if err == nil {
		t.Error("SubmitForm() should return error when no page is loaded")
	}

	// Test data extraction without navigation
	var data struct {
		Title string `find:"h1"`
	}
	err = scraper.ExtractData(&data, "body", UnmarshalOption{})
	if err == nil {
		t.Error("ExtractData() should return error when no page is loaded")
	}
}

// TestUnifiedThreadSafety tests thread safety of the unified interface
func TestUnifiedThreadSafety(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(testHTML))
	}))
	defer server.Close()

	var logger ConsoleLogger
	session := NewSession("test-thread-safety", logger)
	// Set up temporary directory
	tmpDir := t.TempDir()
	session.FilePrefix = tmpDir + "/"
	scraper := UnifiedScraper(session)

	// Navigate once
	err := scraper.DoNavigate(server.URL)
	if err != nil {
		t.Fatalf("Navigate() error = %v", err)
	}

	// Test concurrent SendKeys operations
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(i int) {
			defer func() { done <- true }()

			err := scraper.DoSendKeys("input[name=username]", Sprintf("user%d", i))
			if err != nil {
				t.Errorf("SendKeys() error = %v", err)
			}
		}(i)
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}
}

// TestDownloadOptions tests download options functionality
func TestDownloadOptions(t *testing.T) {
	// Test default timeout setting
	options := UnifiedDownloadOptions{}
	if options.Timeout != 0 {
		t.Errorf("Default timeout should be 0, got %v", options.Timeout)
	}

	// Test with explicit timeout
	options = UnifiedDownloadOptions{
		Timeout: 30 * time.Second,
		Glob:    "*.pdf",
		SaveAs:  "test.pdf",
	}

	// Use the options to avoid unused write warnings
	if options.Glob != "*.pdf" {
		t.Error("Glob should be *.pdf")
	}
	if options.SaveAs != "test.pdf" {
		t.Error("SaveAs should be test.pdf")
	}

	if options.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want %v", options.Timeout, 30*time.Second)
	}
}

// TestScraperTypeDetection tests scraper type detection
func TestScraperTypeDetection(t *testing.T) {
	var logger ConsoleLogger
	session := NewSession("test-type", logger)
	// Set up temporary directory
	tmpDir := t.TempDir()
	session.FilePrefix = tmpDir + "/"

	scraperType := GetScraperType(session)
	if scraperType != HTTPScraper {
		t.Errorf("GetScraperType(session) = %v, want %v", scraperType, HTTPScraper)
	}

	// Test Chrome scraper type detection would require Chrome setup
	// This is left as a placeholder for when Chrome tests are enabled
}

// TestXPathEscaping tests XPath injection prevention
func TestXPathEscaping(t *testing.T) {
	// Test the new proper XPath escaping function
	text := "Test's link"
	escapedText := escapeXPathText(text)
	expected := "\"Test's link\""

	if escapedText != expected {
		t.Errorf("XPath escaping failed: got %q, want %q", escapedText, expected)
	}

	// Test complex case with both quote types
	complexText := "Test's \"complex\" link"
	complexEscaped := escapeXPathText(complexText)
	expectedComplex := "concat('Test', \"'\", 's \"complex\" link')"

	if complexEscaped != expectedComplex {
		t.Errorf("Complex XPath escaping failed: got %q, want %q", complexEscaped, expectedComplex)
	}
}

// Helper function for formatting in concurrent tests
func Sprintf(format string, args ...interface{}) string {
	return fmt.Sprintf(format, args...)
}
