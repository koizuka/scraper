package scraper

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"io/ioutil"
	"os"
	"path"
	"path/filepath"
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
			err := browser.SetDownloadBehavior("allow").WithDownloadPath(downloadPath).Do(ctxt)
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
		err = ioutil.WriteFile(fn, body, 0644)
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
	err = ioutil.WriteFile(responseFilename, jsonData, 0644)
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
