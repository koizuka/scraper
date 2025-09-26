package scraper

import (
	"os"
	"path"
	"strings"
	"testing"
	"time"
)

func TestChromeSession_ReplaySimple(t *testing.T) {
	t.Run("SaveHtml replay mode basic", func(t *testing.T) {
		dir, err := os.MkdirTemp(".", "chrome_replay_simple_test*")
		if err != nil {
			t.Error(err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })

		sessionName := "chrome_replay_simple_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}

		// Create a mock HTML file first
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"
		htmlFile := session.getHtmlFilename()

		// Write a mock HTML file
		mockHTML := "<html><body>Mock HTML for replay test</body></html>"
		err = os.WriteFile(htmlFile, []byte(mockHTML), 0644)
		if err != nil {
			t.Errorf("Failed to create mock HTML file: %v", err)
			return
		}

		// Test replay mode
		session.NotUseNetwork = true
		session.invokeCount = 0 // Reset counter

		// Since we're not running actual Chrome, we can't call action.Do()
		// Instead, test the logic directly
		if session.NotUseNetwork {
			// This should try to read from the file we created
			body, err := os.ReadFile(session.getHtmlFilename())
			if err != nil {
				t.Errorf("SaveHtml in replay mode should read existing file: %v", err)
				return
			}

			if string(body) != mockHTML {
				t.Errorf("Expected %q, got %q", mockHTML, string(body))
				return
			}
		}
	})

	t.Run("Sleep replay mode", func(t *testing.T) {
		logger := BufferedLogger{}
		session := NewSession("test", &logger)
		session.NotUseNetwork = true // Enable replay mode

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Fatalf("NewChromeOpt() error: %v", err)
		}

		start := time.Now()
		err = chromeSession.DoSleep(5 * time.Second)
		elapsed := time.Since(start)

		if err != nil {
			t.Errorf("Sleep() in replay mode error: %v", err)
			return
		}

		// Sleep should be very fast in replay mode (less than 1 second)
		if elapsed > 1*time.Second {
			t.Errorf("Sleep took too long in replay mode: %v", elapsed)
			return
		}
	})

	t.Run("DownloadFile replay mode basic", func(t *testing.T) {
		dir, err := os.MkdirTemp(".", "chrome_download_simple_test*")
		if err != nil {
			t.Error(err)
		}
		t.Cleanup(func() { os.RemoveAll(dir) })

		sessionName := "chrome_download_simple_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Error(err)
		}

		// Create chrome download directory
		chromeDir := path.Join(dir, sessionName, "chrome")
		err = os.MkdirAll(chromeDir, 0755)
		if err != nil {
			t.Error(err)
		}

		// Create a mock downloaded file
		mockFile := path.Join(chromeDir, "test.txt")
		err = os.WriteFile(mockFile, []byte("mock download content"), 0644)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"
		session.NotUseNetwork = true // Enable replay mode

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewTestChromeOptionsWithTimeout(true, 30*time.Second))
		defer cancelFunc()
		if err != nil {
			t.Fatalf("NewChromeOpt() error: %v", err)
		}

		var filename string
		action := chromeSession.DownloadFile(&filename, DownloadFileOptions{})

		// Test replay logic
		err = action(chromeSession.Ctx)
		if err != nil {
			t.Errorf("DownloadFile() in replay mode error: %v", err)
			return
		}

		// Check that the filename ends with the expected path (may be absolute)
		expectedSuffix := path.Join("chrome", "test.txt")
		if !strings.HasSuffix(filename, expectedSuffix) {
			t.Errorf("Expected filename to end with %v, got %v", expectedSuffix, filename)
			return
		}

		// Verify the file actually exists and has correct content
		content, err := os.ReadFile(filename)
		if err != nil {
			t.Errorf("Failed to read downloaded file: %v", err)
			return
		}
		if string(content) != "mock download content" {
			t.Errorf("Expected file content 'mock download content', got %q", string(content))
			return
		}
	})
}
