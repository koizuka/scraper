package scraper

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"strings"
	"testing"
)

func TestSession_DebugStep(t *testing.T) {
	t.Run("SetDebugStep and ClearDebugStep", func(t *testing.T) {
		logger := &BufferedLogger{}
		session := NewSession("test_session", logger)

		// Test initial state (no debug step)
		if session.debugStep != "" {
			t.Errorf("Expected initial debugStep to be empty, got %q", session.debugStep)
		}

		// Test SetDebugStep
		testStep := "テストステップ"
		session.SetDebugStep(testStep)
		if session.debugStep != testStep {
			t.Errorf("Expected debugStep to be %q, got %q", testStep, session.debugStep)
		}

		// Test ClearDebugStep
		session.ClearDebugStep()
		if session.debugStep != "" {
			t.Errorf("Expected debugStep to be empty after clear, got %q", session.debugStep)
		}
	})

	t.Run("getDebugPrefix", func(t *testing.T) {
		logger := &BufferedLogger{}
		session := NewSession("test_session", logger)

		// Test without debug step
		prefix := session.getDebugPrefix()
		expected := "****"
		if prefix != expected {
			t.Errorf("Expected prefix %q, got %q", expected, prefix)
		}

		// Test with debug step
		testStep := "ログイン処理"
		session.SetDebugStep(testStep)
		prefix = session.getDebugPrefix()
		expected = fmt.Sprintf("**** [%s]", testStep)
		if prefix != expected {
			t.Errorf("Expected prefix %q, got %q", expected, prefix)
		}
	})

	t.Run("debug step in SAVE log", func(t *testing.T) {
		// Create a test server
		testContent := "<html><body>Test content</body></html>"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, testContent)
		}))
		defer ts.Close()

		// Create temp directory
		dir, err := os.MkdirTemp(".", "session_test*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		sessionName := "debug_test_session"
		err = os.Mkdir(path.Join(dir, sessionName), 0744)
		if err != nil {
			t.Fatal(err)
		}

		logger := &BufferedLogger{}
		session := NewSession(sessionName, logger)
		session.FilePrefix = dir + "/"
		session.SaveToFile = true

		// Test without debug step
		_, err = session.Get(ts.URL)
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}

		logOutput := logger.buffer.String()
		if !strings.Contains(logOutput, "**** SAVE to") {
			t.Errorf("Expected default SAVE log format, got: %s", logOutput)
		}

		// Clear logger buffer
		logger.buffer.Reset()

		// Test with debug step
		debugStep := "データ取得"
		session.SetDebugStep(debugStep)
		_, err = session.Get(ts.URL)
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}

		logOutput = logger.buffer.String()
		expectedLog := fmt.Sprintf("**** [%s] SAVE to", debugStep)
		if !strings.Contains(logOutput, expectedLog) {
			t.Errorf("Expected debug step in SAVE log %q, got: %s", expectedLog, logOutput)
		}
	})

	t.Run("debug step in LOAD log", func(t *testing.T) {
		// Create temp directory and test file
		dir, err := os.MkdirTemp(".", "session_test*")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(dir)

		sessionName := "debug_load_test_session"
		sessionDir := path.Join(dir, sessionName)
		err = os.Mkdir(sessionDir, 0744)
		if err != nil {
			t.Fatal(err)
		}

		// Create a test HTML file
		testContent := "<html><body>Test content</body></html>"
		testFile := path.Join(sessionDir, "1.html")
		err = os.WriteFile(testFile, []byte(testContent), 0644)
		if err != nil {
			t.Fatal(err)
		}

		// Create metadata file using helper function
		metadata := PageMetadata{
			URL:         "http://example.com",
			ContentType: "text/html",
		}
		err = savePageMetadata(testFile, metadata)
		if err != nil {
			t.Fatal(err)
		}

		logger := &BufferedLogger{}
		session := NewSession(sessionName, logger)
		session.FilePrefix = dir + "/"
		session.NotUseNetwork = true // Force loading from file

		// Test with debug step
		debugStep := "ファイル読み込み"
		session.SetDebugStep(debugStep)

		_, err = session.Get("http://example.com")
		if err != nil {
			t.Fatalf("Get() error: %v", err)
		}

		logOutput := logger.buffer.String()
		expectedLog := fmt.Sprintf("**** [%s] LOAD from", debugStep)
		if !strings.Contains(logOutput, expectedLog) {
			t.Errorf("Expected debug step in LOAD log %q, got: %s", expectedLog, logOutput)
		}
	})

	t.Run("SetDebugStep logs START message", func(t *testing.T) {
		logger := &BufferedLogger{}
		session := NewSession("test_session", logger)

		testStep := "テスト処理"
		session.SetDebugStep(testStep)

		logOutput := logger.buffer.String()
		expectedLog := fmt.Sprintf("**** [%s] START\n", testStep)
		if logOutput != expectedLog {
			t.Errorf("Expected log %q, got %q", expectedLog, logOutput)
		}
	})

	t.Run("ClearDebugStep logs END message", func(t *testing.T) {
		logger := &BufferedLogger{}
		session := NewSession("test_session", logger)

		// Set debug step first
		testStep := "テスト処理"
		session.SetDebugStep(testStep)

		// Clear buffer to test only ClearDebugStep output
		logger.buffer.Reset()

		// Clear debug step
		session.ClearDebugStep()

		logOutput := logger.buffer.String()
		expectedLog := fmt.Sprintf("**** [%s] END\n", testStep)
		if logOutput != expectedLog {
			t.Errorf("Expected log %q, got %q", expectedLog, logOutput)
		}
	})

	t.Run("ClearDebugStep does not log when no debug step is set", func(t *testing.T) {
		logger := &BufferedLogger{}
		session := NewSession("test_session", logger)

		// Clear debug step without setting it first
		session.ClearDebugStep()

		logOutput := logger.buffer.String()
		if logOutput != "" {
			t.Errorf("Expected no log output, got %q", logOutput)
		}
	})
}
