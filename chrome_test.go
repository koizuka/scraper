package scraper

import (
	"fmt"
	"github.com/google/go-cmp/cmp"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
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

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewChromeOptions{Headless: true, Timeout: 30 * time.Second})
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

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewChromeOptions{Headless: true, Timeout: 10 * time.Second})
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
