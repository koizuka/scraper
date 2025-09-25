package scraper

import (
	"context"
	"errors"
	"os"
	"path"
	"testing"
)

func TestChromeSession_ReplayExtended(t *testing.T) {
	t.Run("SaveFile replay mode", func(t *testing.T) {
		dir, err := os.MkdirTemp(".", "chrome_savefile_replay_test*")
		if err != nil {
			t.Error(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_savefile_replay_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"
		session.NotUseNetwork = true // Enable replay mode

		chromeSession := &ChromeSession{Session: session}

		// Create the expected saved file
		session.invokeCount = 1
		expectedFile := session.getHtmlFilename()
		expectedFile = expectedFile[:len(expectedFile)-5] + ".txt" // Change extension to .txt
		err = os.WriteFile(expectedFile, []byte("test content"), 0644)
		if err != nil {
			t.Error(err)
		}

		// Reset counter for test
		session.invokeCount = 0

		// Test SaveFile in replay mode
		sourceFile := path.Join(dir, "source.txt")
		err = os.WriteFile(sourceFile, []byte("source content"), 0644)
		if err != nil {
			t.Error(err)
		}

		action := chromeSession.SaveFile(&sourceFile)
		err = action(context.Background())
		if err != nil {
			t.Errorf("SaveFile() in replay mode error: %v", err)
			return
		}

		// Verify the saved file still exists
		if _, err := os.Stat(expectedFile); os.IsNotExist(err) {
			t.Errorf("Expected saved file should exist: %v", expectedFile)
			return
		}
	})

	t.Run("RetryAndRecordError on missing files", func(t *testing.T) {
		dir, err := os.MkdirTemp(".", "chrome_retry_error_test*")
		if err != nil {
			t.Error(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_retry_error_test"
		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"
		session.NotUseNetwork = true // Enable replay mode

		chromeSession := &ChromeSession{Session: session}

		// Test SaveHtml with missing file
		_, err = chromeSession.actionChrome(chromeSession.SaveHtml(nil))

		var retryErr RetryAndRecordError
		if err == nil {
			t.Errorf("Expected RetryAndRecordError, got nil")
			return
		}
		if !errors.As(err, &retryErr) {
			t.Errorf("Expected RetryAndRecordError, got %T: %v", err, err)
			return
		}
	})

	t.Run("HTML counter increment test", func(t *testing.T) {
		dir, err := os.MkdirTemp(".", "chrome_counter_test*")
		if err != nil {
			t.Error(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_counter_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"
		session.NotUseNetwork = true // Enable replay mode

		chromeSession := &ChromeSession{Session: session}

		// Create multiple mock HTML files
		for i := 1; i <= 3; i++ {
			session.invokeCount = i
			htmlFile := session.getHtmlFilename()
			mockHTML := `<html><body>Mock HTML ` + string(rune('0'+i)) + `</body></html>`
			err = os.WriteFile(htmlFile, []byte(mockHTML), 0644)
			if err != nil {
				t.Error(err)
			}
		}

		// Reset counter
		session.invokeCount = 0

		// Call SaveHtml multiple times and verify counter increments
		for i := 1; i <= 3; i++ {
			_, err = chromeSession.actionChrome(chromeSession.SaveHtml(nil))
			if err != nil {
				t.Errorf("SaveHtml() call %d error: %v", i, err)
				return
			}

			if session.invokeCount != i {
				t.Errorf("Expected invokeCount %d, got %d", i, session.invokeCount)
				return
			}
		}
	})

	t.Run("Unified interface replay mode", func(t *testing.T) {
		dir, err := os.MkdirTemp(".", "chrome_unified_replay_test*")
		if err != nil {
			t.Error(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_unified_replay_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"
		session.NotUseNetwork = true // Enable replay mode

		chromeSession := &ChromeSession{Session: session}

		// Create mock HTML files for each operation
		for i := 1; i <= 4; i++ {
			session.invokeCount = i
			htmlFile := session.getHtmlFilename()
			mockHTML := `<html><body>Mock HTML for operation ` + string(rune('0'+i)) + `</body></html>`
			err = os.WriteFile(htmlFile, []byte(mockHTML), 0644)
			if err != nil {
				t.Error(err)
			}
		}

		// Reset counter
		session.invokeCount = 0

		// Test unified interface methods in replay mode
		operations := []struct {
			name string
			op   func() error
		}{
			{"Navigate", func() error { return chromeSession.Navigate("http://example.com") }},
			{"WaitVisible", func() error { return chromeSession.WaitVisible("#test") }},
			{"Click", func() error { return chromeSession.Click("#button") }},
			{"SendKeys", func() error { return chromeSession.SendKeys("#input", "test") }},
		}

		for i, test := range operations {
			err := test.op()
			if err != nil {
				t.Errorf("%s operation failed: %v", test.name, err)
				return
			}

			expectedCount := i + 1
			if session.invokeCount != expectedCount {
				t.Errorf("After %s: expected invokeCount %d, got %d", test.name, expectedCount, session.invokeCount)
				return
			}
		}
	})

	t.Run("SubmitForm and FollowAnchor replay mode", func(t *testing.T) {
		dir, err := os.MkdirTemp(".", "chrome_form_anchor_replay_test*")
		if err != nil {
			t.Error(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_form_anchor_replay_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"
		session.NotUseNetwork = true // Enable replay mode

		chromeSession := &ChromeSession{Session: session}

		// Create mock HTML files
		for i := 1; i <= 2; i++ {
			session.invokeCount = i
			htmlFile := session.getHtmlFilename()
			mockHTML := `<html><body>Mock HTML for form/anchor operation ` + string(rune('0'+i)) + `</body></html>`
			err = os.WriteFile(htmlFile, []byte(mockHTML), 0644)
			if err != nil {
				t.Error(err)
			}
		}

		// Reset counter
		session.invokeCount = 0

		// Test SubmitForm in replay mode - expect it to fail without navigation
		err = chromeSession.SubmitForm("#form", map[string]string{"input": "value"})
		if err == nil {
			t.Errorf("SubmitForm() should fail in replay mode without navigation")
			return
		}

		// Test FollowAnchor in replay mode - expect it to fail without navigation
		err = chromeSession.FollowAnchor("Click me")
		if err == nil {
			t.Errorf("FollowAnchor() should fail in replay mode without navigation")
			return
		}
	})

	t.Run("DownloadFile glob pattern test", func(t *testing.T) {
		dir, err := os.MkdirTemp(".", "chrome_download_glob_test*")
		if err != nil {
			t.Error(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_download_glob_test"
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

		// Create multiple mock downloaded files
		files := []string{"data.csv", "report.txt", "image.png"}
		for _, fileName := range files {
			mockFile := path.Join(chromeDir, fileName)
			err = os.WriteFile(mockFile, []byte("mock content for "+fileName), 0644)
			if err != nil {
				t.Error(err)
			}
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"
		session.NotUseNetwork = true // Enable replay mode

		chromeSession := &ChromeSession{
			Session:      session,
			DownloadPath: chromeDir,
		}

		// Test with specific glob pattern
		var filename string
		action := chromeSession.DownloadFile(&filename, DownloadFileOptions{Glob: "*.csv"})
		err = action(nil)
		if err != nil {
			t.Errorf("DownloadFile() with CSV glob error: %v", err)
			return
		}

		expectedFilename := path.Join(chromeDir, "data.csv")
		if filename != expectedFilename {
			t.Errorf("Expected filename %v, got %v", expectedFilename, filename)
			return
		}
	})

	t.Run("Context cancellation in replay mode", func(t *testing.T) {
		dir, err := os.MkdirTemp(".", "chrome_context_cancel_test*")
		if err != nil {
			t.Error(err)
		}
		defer func() { _ = os.RemoveAll(dir) }()

		sessionName := "chrome_context_cancel_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"
		session.NotUseNetwork = true // Enable replay mode

		chromeSession := &ChromeSession{Session: session}

		// Create a cancelled context
		_, cancel := context.WithCancel(context.Background())
		cancel() // Immediately cancel

		// Create mock HTML file
		session.invokeCount = 1
		htmlFile := session.getHtmlFilename()
		err = os.WriteFile(htmlFile, []byte("<html><body>test</body></html>"), 0644)
		if err != nil {
			t.Error(err)
		}

		// Reset counter
		session.invokeCount = 0

		// SaveHtml should work in replay mode even with cancelled context
		// because it doesn't perform browser operations
		_, err = chromeSession.actionChrome(chromeSession.SaveHtml(nil))
		if err != nil {
			t.Errorf("SaveHtml should work in replay mode even with cancelled context: %v", err)
			return
		}

		if session.invokeCount != 1 {
			t.Errorf("Expected invokeCount 1, got %d", session.invokeCount)
			return
		}
	})
}
