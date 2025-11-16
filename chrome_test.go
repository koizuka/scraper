package scraper

import (
	"fmt"
	"github.com/chromedp/chromedp"
	"github.com/google/go-cmp/cmp"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

func TestSession_RunNavigate(t *testing.T) {
	t.Run("got html", func(t *testing.T) {
		// create a test server to serve the page
		shouldBe := "<html><head></head><body>Hello World.</body></html>"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, shouldBe)
		}))
		defer ts.Close()

		dir, err := os.MkdirTemp(".", "chrome_test*")
		if err != nil {
			t.Error(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Errorf("NewChromeOpt() error: %v", err)
			return
		}

		if _, err := chromeSession.RunNavigate(ts.URL); err != nil {
			t.Errorf("RunNavigate() error: %v", err)
			return
		}

		filename := session.getHtmlFilename()
		rawHtml, err := os.ReadFile(filename)
		if err != nil {
			t.Error(err)
			return
		}
		html := string(rawHtml)
		if diff := cmp.Diff(shouldBe, html); diff != "" {
			t.Errorf("(-shouldBe +got)\n%v", diff)
			return
		}
	})

	t.Run("not found", func(t *testing.T) {
		// create a test server to serve the page
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			status := 404
			http.Error(w, http.StatusText(status), status)
		}))
		defer ts.Close()

		dir, err := os.MkdirTemp(".", "chrome_test*")
		if err != nil {
			t.Error(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 10*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Errorf("NewChromeOpt() error: %v", err)
			return
		}

		resp, err := chromeSession.RunNavigate(ts.URL)
		if err != nil {
			t.Errorf("RunNavigate() error: %v", err)
			return
		}
		type values struct {
			Status int64
		}
		got := values{
			Status: resp.Status,
		}
		shouldBe := values{
			Status: 404,
		}

		if diff := cmp.Diff(shouldBe, got); diff != "" {
			t.Errorf("(-shouldBe +got)\n%v", diff)
			return
		}
	})
}

func TestChromeSession_DownloadFile(t *testing.T) {
	// create a test server to serve file download request
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Disposition", "attachment; filename=example.txt")
		_, _ = fmt.Fprint(w, "Hello World.")
	}))

	dir, err := os.MkdirTemp(".", "chrome_test*")
	if err != nil {
		t.Error(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	sessionName := "chrome_test"
	err = os.Mkdir(path.Join(dir, sessionName), 0744)
	if err != nil {
		t.Error(err)
	}

	logger := BufferedLogger{}
	session := NewSession(sessionName, &logger)

	tests := []struct {
		name    string
		opt     DownloadFileOptions
		wantErr bool
	}{
		{
			name:    "no glob",
			opt:     DownloadFileOptions{},
			wantErr: false,
		},
		{
			name:    "invalid glob",
			opt:     DownloadFileOptions{Glob: "/"},
			wantErr: true,
		},
		{
			name:    "valid glob",
			opt:     DownloadFileOptions{Glob: "*"},
			wantErr: false,
		},
		{
			name:    "valid but not matched glob",
			opt:     DownloadFileOptions{Glob: "*.html"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
			defer cancelFunc()
			if err != nil {
				t.Errorf("NewChromeOpt() error: %v", err)
				return
			}

			// download file
			var downloadedFilename string
			err = chromedp.Run(chromeSession.Ctx,
				chromeSession.DownloadFile(&downloadedFilename, tt.opt,
					chromedp.Navigate(ts.URL),
				),
			)
			if (err != nil) != tt.wantErr {
				t.Errorf("DownloadFile() error: %v", err)
				return
			}

			if !tt.wantErr {
				rawFile, err := os.ReadFile(downloadedFilename)
				if err != nil {
					t.Error(err)
					return
				}

				file := string(rawFile)
				if diff := cmp.Diff("Hello World.", file); diff != "" {
					t.Errorf("(-shouldBe +got)\n%v", diff)
					return
				}
			}
		})
	}
}

func TestChromeSession_DebugStep(t *testing.T) {
	t.Run("debug step in Chrome SAVE log", func(t *testing.T) {
		// Create a test server
		testContent := "<html><body>Chrome Test Content</body></html>"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, testContent)
		}))
		defer ts.Close()

		// Create temp directory
		dir, err := os.MkdirTemp(".", "chrome_debug_test*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		sessionName := "chrome_debug_test_session"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Fatal(err)
		}

		logger := &BufferedLogger{}
		session := NewSession(sessionName, logger)
		session.FilePrefix = dir + "/"

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Fatalf("NewChromeOpt() error: %v", err)
		}

		// Test with debug step
		debugStep := "Chrome ナビゲート"
		chromeSession.SetDebugStep(debugStep)

		_, err = chromeSession.RunNavigate(ts.URL)
		if err != nil {
			t.Fatalf("RunNavigate() error: %v", err)
		}

		logOutput := logger.buffer.String()
		expectedLog := fmt.Sprintf("**** [%s] SAVE to", debugStep)
		if !strings.Contains(logOutput, expectedLog) {
			t.Errorf("Expected debug step in Chrome SAVE log %q, got: %s", expectedLog, logOutput)
		}
	})

	t.Run("debug step inheritance from Session", func(t *testing.T) {
		// Create temp directory
		dir, err := os.MkdirTemp(".", "chrome_debug_inherit_test*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		sessionName := "chrome_debug_inherit_test_session"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Fatal(err)
		}

		logger := &BufferedLogger{}
		session := NewSession(sessionName, logger)
		session.FilePrefix = dir + "/"

		// Set debug step on Session
		debugStep := "継承テスト"
		session.SetDebugStep(debugStep)

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Fatalf("NewChromeOpt() error: %v", err)
		}

		// Verify ChromeSession inherits the debug step from Session
		if chromeSession.debugStep != debugStep {
			t.Errorf("Expected ChromeSession to inherit debug step %q, got %q", debugStep, chromeSession.debugStep)
		}

		// Test clearing debug step from ChromeSession affects the embedded Session
		chromeSession.ClearDebugStep()
		if session.debugStep != "" {
			t.Errorf("Expected Session debug step to be cleared via ChromeSession, got %q", session.debugStep)
		}
	})
}

func TestChromeSession_ReplayMode(t *testing.T) {
	// Create a test server with multiple pages
	testPages := map[string]string{
		"/":       "<html><body><h1>Home Page</h1><a href='/page2'>Go to Page 2</a></body></html>",
		"/page2":  "<html><body><h1>Page 2</h1><button id='btn'>Click me</button></body></html>",
		"/result": "<html><body><h1>Result Page</h1><p>Success!</p></body></html>",
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if content, exists := testPages[r.URL.Path]; exists {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, content)
		} else {
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	// Create temp directory
	dir, err := os.MkdirTemp(".", "chrome_replay_test*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	sessionName := "chrome_replay_test"
	err = os.Mkdir(path.Join(dir, sessionName), 0744)
	if err != nil {
		t.Fatal(err)
	}

	logger := &BufferedLogger{}
	session := NewSession(sessionName, logger)
	session.FilePrefix = dir + "/"

	t.Run("record mode", func(t *testing.T) {
		// First, record the session
		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Fatalf("NewChromeOpt() error: %v", err)
		}

		// Record a series of actions
		err = chromeSession.DoNavigate(ts.URL)
		if err != nil {
			t.Errorf("Navigate() error: %v", err)
		}

		err = chromeSession.DoWaitVisible("h1")
		if err != nil {
			t.Errorf("WaitVisible() error: %v", err)
		}

		err = chromeSession.DoClick("a")
		if err != nil {
			t.Errorf("Click() error: %v", err)
		}

		err = chromeSession.DoWaitVisible("#btn")
		if err != nil {
			t.Errorf("WaitVisible() error: %v", err)
		}

		// Verify files were created
		expectedFiles := []string{
			path.Join(dir, sessionName, "1.html"),
			path.Join(dir, sessionName, "2.html"),
			path.Join(dir, sessionName, "3.html"),
			path.Join(dir, sessionName, "4.html"),
		}

		for _, expectedFile := range expectedFiles {
			if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
				t.Errorf("Expected file %s was not created", expectedFile)
			}
		}
	})

	t.Run("replay mode", func(t *testing.T) {
		// Reset logger and invoke count for replay test
		logger.buffer.Reset()
		session.invokeCount = 0

		// Enable replay mode
		session.NotUseNetwork = true

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Fatalf("NewChromeOpt() error: %v", err)
		}

		// Replay the same series of actions
		err = chromeSession.DoNavigate("http://should-not-be-used")
		if err != nil {
			t.Errorf("Navigate() in replay mode error: %v", err)
		}

		err = chromeSession.DoWaitVisible("h1")
		if err != nil {
			t.Errorf("WaitVisible() in replay mode error: %v", err)
		}

		err = chromeSession.DoClick("a")
		if err != nil {
			t.Errorf("Click() in replay mode error: %v", err)
		}

		err = chromeSession.DoWaitVisible("#btn")
		if err != nil {
			t.Errorf("WaitVisible() in replay mode error: %v", err)
		}

		// Test that ExtractData works in replay mode
		type PageData struct {
			Title string `find:"h1"`
		}

		var data PageData
		err = chromeSession.ExtractData(&data, "body", UnmarshalOption{})
		if err != nil {
			t.Errorf("ExtractData() in replay mode error: %v", err)
		}

		// The last page should be page2 with "Page 2" title
		expectedTitle := "Page 2"
		if data.Title != expectedTitle {
			t.Errorf("Expected title %q, got %q", expectedTitle, data.Title)
		}

		// Verify that replay logs were generated
		logOutput := logger.buffer.String()
		if !strings.Contains(logOutput, "REPLAY LOADED") {
			t.Errorf("Expected replay logs, got: %s", logOutput)
		}
	})

	t.Run("replay mode - file not found", func(t *testing.T) {
		// Reset for clean test
		session.invokeCount = 0
		session.NotUseNetwork = true

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Fatalf("NewChromeOpt() error: %v", err)
		}

		// Remove one of the saved files to test error handling
		os.Remove(path.Join(dir, sessionName, "1.html"))

		err = chromeSession.DoNavigate("http://should-not-be-used")
		if err == nil {
			t.Error("Expected error when replay file not found, got nil")
		}

		// Check that it returns RetryAndRecordError
		if _, ok := err.(RetryAndRecordError); !ok {
			t.Errorf("Expected RetryAndRecordError, got %T: %v", err, err)
		}
	})
}

func TestChromeSession_SaveLastHtmlSnapshot(t *testing.T) {
	t.Run("captures and saves HTML snapshot", func(t *testing.T) {
		// Create a simple test server
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, "<html><body><h1>Test Page</h1></body></html>")
		}))
		defer ts.Close()

		dir, err := os.MkdirTemp(".", "chrome_snapshot_test*")
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_snapshot_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Fatal(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Fatalf("NewChromeOpt() error: %v", err)
		}

		// Navigate to page
		err = chromeSession.DoNavigate(ts.URL)
		if err != nil {
			t.Fatalf("Navigate() error: %v", err)
		}

		// Manually capture HTML (simulating what happens before operations)
		var html string
		err = chromedp.Run(chromeSession.Ctx, chromedp.OuterHTML("html", &html, chromedp.ByQuery))
		if err != nil {
			t.Fatalf("OuterHTML() error: %v", err)
		}
		chromeSession.lastHtml = html

		// Verify lastHtml contains expected content
		if !strings.Contains(chromeSession.lastHtml, "Test Page") {
			t.Error("lastHtml should contain 'Test Page'")
		}

		// Save snapshot
		err = chromeSession.SaveLastHtmlSnapshot()
		if err != nil {
			t.Fatalf("SaveLastHtmlSnapshot() error: %v", err)
		}

		// Verify snapshot file exists and contains correct content
		snapshotPath := chromeSession.GetSnapshotFilename()
		content, err := os.ReadFile(snapshotPath)
		if err != nil {
			t.Fatalf("Failed to read snapshot: %v", err)
		}
		if !strings.Contains(string(content), "Test Page") {
			t.Error("Snapshot should contain 'Test Page'")
		}
	})

	t.Run("handles empty lastHtml gracefully", func(t *testing.T) {
		logger := BufferedLogger{}
		session := NewSession("test", &logger)

		chromeSession := &ChromeSession{
			Session:  session,
			lastHtml: "", // Empty
		}

		// Should not error, just do nothing
		err := chromeSession.SaveLastHtmlSnapshot()
		if err != nil {
			t.Errorf("SaveLastHtmlSnapshot() with empty lastHtml should not error, got: %v", err)
		}
	})
}
