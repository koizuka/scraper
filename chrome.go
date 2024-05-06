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
		browser.SetDownloadBehavior("allow").
			WithDownloadPath(downloadPath).
			WithEventsEnabled(true),
	)
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

		downloadCtx, cancel := context.WithTimeout(ctxt, 5*time.Second)
		defer cancel()

		startTime := time.Now()

		suggestedFilename := ""
		chromedp.ListenTarget(downloadCtx, func(ev interface{}) {
			switch ev := ev.(type) {
			case *browser.EventDownloadWillBegin:
				suggestedFilename = path.Join(session.DownloadPath, ev.SuggestedFilename)
			case *browser.EventDownloadProgress:
				switch ev.State {
				case browser.DownloadProgressStateCompleted:
					download <- suggestedFilename
				case browser.DownloadProgressStateCanceled:
					download <- ""
				}
			}
		})
		err := chromedp.Run(ctxt, actions...)
		if err != nil && !strings.Contains(err.Error(), "net::ERR_ABORTED") {
			return err
		}

		select {
		case <-downloadCtx.Done():
			// browser.DownloadProgressStateCompleted が来なかった場合、新しいファイルがあったらそれを返す
			files, err := os.ReadDir(session.DownloadPath)
			if err != nil {
				return err
			}
			for _, file := range files {
				info, err := file.Info()
				if err != nil {
					return err
				}
				if !info.ModTime().Before(startTime) {
					*filename = path.Join(session.DownloadPath, file.Name())
					session.Printf("**** DOWNLOADED: %v\n", *filename)
					return nil
				}
			}
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

/**
 * SaveFile saves file to filename
 * filename: DownloadFile の結果を chromedp.Run で続ける場合、ポインタにしないと実行前の値が渡ってしまうため、ポインタにする
 */
func (session *ChromeSession) SaveFile(filename *string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		session.invokeCount++
		if filename == nil {
			return fmt.Errorf("filename is nil")
		}

		fn := session.getHtmlFilename()

		// change extension with filename
		fn = strings.TrimSuffix(fn, filepath.Ext(fn)) + filepath.Ext(*filename)

		body, err := os.ReadFile(*filename)
		if err != nil {
			return err
		}
		err = os.WriteFile(fn, body, 0644)
		if err != nil {
			return err
		}
		session.Printf("**** SAVE %v to %v (%v bytes)\n", *filename, fn, len(body))
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
