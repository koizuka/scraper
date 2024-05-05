package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

type ChromeSession struct {
	*Session
	Ctx          context.Context
	DownloadPath string
}

type NewChromeOptions struct {
	Headless bool
	Timeout  time.Duration
}

func (session *Session) NewChromeOpt(options NewChromeOptions) (chromeSession *ChromeSession, cancelFunc context.CancelFunc, err error) {
	chromeUserDataDir, err := filepath.Abs("./chromeUserData")
	if err != nil {
		return nil, func() {}, err
	}

	allocOptions := []chromedp.ExecAllocatorOption{
		chromedp.UserDataDir(chromeUserDataDir),
	}
	if options.Headless {
		allocOptions = append(allocOptions,
			chromedp.Headless,
			chromedp.DisableGPU,
		)
	}

	downloadPath, err := filepath.Abs(path.Join(session.getDirectory(), "chrome"))
	if err != nil {
		return nil, func() {}, err
	}

	err = os.MkdirAll(downloadPath, 0777)
	if err != nil {
		return nil, func() {}, fmt.Errorf("couldn't create directory: %v", downloadPath)
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOptions...)

	ctxt, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(session.Printf))
	if options.Timeout != 0 {
		ctxt, cancel = context.WithTimeout(ctxt, options.Timeout)
	}

	chromeSession = &ChromeSession{session, ctxt, downloadPath}

	cancelFunc = func() {
		cancel()
		allocCancel()
	}

	// configure to download behavior
	err = chromedp.Run(ctxt,
		chromedp.ActionFunc(func(ctxt context.Context) error {
			err := (&browser.SetDownloadBehaviorParams{
				Behavior:      "allow",
				DownloadPath:  downloadPath,
				EventsEnabled: true,
			}).Do(ctxt)
			if err != nil {
				return err
			}
			return nil
		}))

	if err != nil {
		return chromeSession, cancelFunc, err
	}

	return chromeSession, cancelFunc, nil
}

func (session *Session) NewChrome() (*ChromeSession, context.CancelFunc, error) {
	return session.NewChromeOpt(NewChromeOptions{Headless: false})
}

func (session *ChromeSession) SaveHtml(filename *string) chromedp.Action {
	return chromedp.ActionFunc(func(ctxt context.Context) error {
		var html string
		err := chromedp.OuterHTML("html", &html, chromedp.ByQuery).Do(ctxt)
		if err != nil {
			return err
		}

		session.invokeCount++
		fn := session.getHtmlFilename()
		body := []byte(html)
		err = os.WriteFile(fn, body, 0644)
		if err != nil {
			return err
		}
		if filename != nil {
			*filename = fn
		}
		session.Printf("**** SAVE to %v (%v bytes)\n", fn, len(body))
		var title string
		err = chromedp.Title(&title).Do(ctxt)
		if err == nil {
			session.Printf("* %v\n", title)
		}
		return nil
	})
}

func (session *ChromeSession) DownloadFile(filename *string, actions ...chromedp.Action) chromedp.ActionFunc {
	return func(ctxt context.Context) error {
		if filename == nil {
			return fmt.Errorf("filename is nil")
		}
		download := make(chan string)

		downloadCtx, cancel := context.WithTimeout(ctxt, 10*time.Second)
		defer cancel()

		suggestedFilename := ""
		chromedp.ListenTarget(downloadCtx, func(ev interface{}) {
			if begin, ok := ev.(*browser.EventDownloadWillBegin); ok {
				suggestedFilename = path.Join(session.DownloadPath, begin.SuggestedFilename)
			} else if progress, ok := ev.(*browser.EventDownloadProgress); ok {
				switch progress.State {
				case browser.DownloadProgressStateCompleted:
					download <- suggestedFilename

				case browser.DownloadProgressStateCanceled:
					session.Printf("**** DOWNLOAD CANCELED\n")
					download <- ""
				}
			}
			if ev, ok := ev.(*network.EventResponseReceived); ok {
				if ev.Response.URL == *filename {
					session.Printf("**** DOWNLOAD %v\n", *filename)
				}
			}
		})
		err := chromedp.Run(ctxt, actions...)
		if err != nil && !strings.Contains(err.Error(), "net::ERR_ABORTED") {
			return err
		}

		select {
		case <-downloadCtx.Done():
			return downloadCtx.Err()
		case downloaded := <-download:
			if downloaded == "" {
				return fmt.Errorf("download canceled")
			}
			*filename = downloaded
			session.Printf("**** DOWNLOADED: %v\n", *filename)
			return nil
		}
	}
}

func (session *ChromeSession) SaveFile(filename string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		session.invokeCount++
		fn := session.getHtmlFilename()
		body, err := os.ReadFile(filename)
		if err != nil {
			return err
		}
		err = os.WriteFile(fn, body, 0644)
		if err != nil {
			return err
		}
		session.Printf("**** SAVE to %v (%v bytes)\n", fn, len(body))
		return nil
	}
}

func (session *ChromeSession) actionChrome(action chromedp.Action) (*network.Response, error) {
	var filename string
	resp, err := chromedp.RunResponse(session.Ctx, chromedp.Tasks{
		action,
		session.SaveHtml(&filename),
	})
	if err != nil {
		return nil, err
	}

	responseFilename := filename + ".response.json"

	jsonData, err := json.Marshal(resp)
	err = os.WriteFile(responseFilename, jsonData, 0644)
	if err != nil {
		return nil, err
	}

	return resp, nil
}

// RunNavigate navigates to page URL and download html like Session.invoke
func (session *ChromeSession) RunNavigate(URL string) (*network.Response, error) {
	return session.actionChrome(chromedp.Navigate(URL))
}

func (session *ChromeSession) Unmarshal(v interface{}, cssSelector string, opt UnmarshalOption) error {
	return ChromeUnmarshal(session.Ctx, v, cssSelector, opt)
}
