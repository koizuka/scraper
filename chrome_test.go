package scraper

import (
	"fmt"
	"github.com/chromedp/chromedp"
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

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewChromeOptions{
			Headless:          true,
			Timeout:           30 * time.Second,
			ExtraAllocOptions: getCICompatibleChromeOptions(),
		})
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

		chromeSession, cancelFunc, err := session.NewChromeOpt(NewChromeOptions{
			Headless:          true,
			Timeout:           10 * time.Second,
			ExtraAllocOptions: getCICompatibleChromeOptions(),
		})
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
			chromeSession, cancelFunc, err := session.NewChromeOpt(NewChromeOptions{
				Headless:          true,
				Timeout:           30 * time.Second,
				ExtraAllocOptions: getCICompatibleChromeOptions(),
			})
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
