package scraper

import (
	"context"
	"fmt"
	"github.com/chromedp/chromedp"
	"github.com/google/go-cmp/cmp"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"time"
)

func TestSession_NavigateChrome(t *testing.T) {
	t.Run("got html", func(t *testing.T) {
		// create a test server to serve the page
		shouldBe := "<html><head></head><body>Hello World.</body></html>"
		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = fmt.Fprint(w, shouldBe)
		}))
		defer ts.Close()

		// create context
		ctx, cancel := chromedp.NewContext(context.Background())
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		dir, err := ioutil.TempDir(".", "chrome_test*")
		if err != nil {
			t.Error(err)
		}
		defer os.RemoveAll(dir)

		sessionName := "chrome_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0644)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"

		if _, err := session.NavigateChrome(ctx, ts.URL); err != nil {
			t.Errorf("NavigateChrome() error = %v", err)
			return
		}

		filename := session.getHtmlFilename()
		rawHtml, err := ioutil.ReadFile(filename)
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

		// create context
		ctx, cancel := chromedp.NewContext(context.Background())
		ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
		defer cancel()

		dir, err := ioutil.TempDir(".", "chrome_test*")
		if err != nil {
			t.Error(err)
		}
		defer os.RemoveAll(dir)

		sessionName := "chrome_test"
		err = os.Mkdir(path.Join(dir, sessionName), 0644)
		if err != nil {
			t.Error(err)
		}

		logger := BufferedLogger{}
		session := NewSession(sessionName, &logger)
		session.FilePrefix = dir + "/"

		resp, err := session.NavigateChrome(ctx, ts.URL)
		if err != nil {
			t.Errorf("NavigateChrome() error = %v", err)
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
