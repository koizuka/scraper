//go:build integration

package scraper

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/chromedp/chromedp"
)

// Browser-level subscriptions must survive popup downloads, and a failed
// trigger must release its subscriptions before the next download starts.
func TestChromeSession_DownloadFile_PopupAfterFailedTrigger(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/download" {
			w.Header().Set("Content-Disposition", "attachment; filename=popup.txt")
			fmt.Fprint(w, "popup download")
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprint(w, `<button id="download" onclick="window.open('/download')">Download</button>`)
	}))
	defer ts.Close()

	session := NewSession("popup", &BufferedLogger{})
	session.FilePrefix = t.TempDir() + string(os.PathSeparator)
	if err := os.MkdirAll(session.getDirectory(), 0755); err != nil {
		t.Fatal(err)
	}
	chrome, cancel, err := NewChromeWithRetry(session, newIsolatedTestChromeOptions(t, true, 30*time.Second), 2)
	defer cancel()
	if err != nil {
		t.Fatal(err)
	}
	if err := chromedp.Do(chrome.Ctx, chromedp.Navigate(ts.URL)); err != nil {
		t.Fatal(err)
	}

	var filename string
	triggerErr := errors.New("trigger failed")
	err = chromedp.Do(chrome.Ctx, chrome.DownloadFile(&filename, DownloadFileOptions{},
		chromedp.Func(func(context.Context, *chromedp.Target) error { return triggerErr }),
	))
	if !errors.Is(err, triggerErr) {
		t.Fatalf("failed trigger: got %v, want %v", err, triggerErr)
	}

	err = chromedp.Do(chrome.Ctx, chrome.DownloadFile(&filename,
		DownloadFileOptions{Glob: "*.txt", Timeout: 15 * time.Second},
		chromedp.Click(chromedp.CSS("#download")),
	))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(filename) != "popup.txt" {
		t.Fatalf("download filename = %q, want popup.txt", filename)
	}
	data, err := os.ReadFile(filename)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "popup download" {
		t.Fatalf("download contents = %q", data)
	}
}
