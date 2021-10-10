package scraper

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"os"
	"path/filepath"
	"time"
)

type NewChromeOptions struct {
	Headless bool
	Timeout  time.Duration
}

func (session *Session) NewChromeOpt(options NewChromeOptions) (context.Context, context.CancelFunc, string, error) {
	chromeUserDataDir, err := filepath.Abs("./chromeUserData")
	if err != nil {
		return nil, func() {}, "", err
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

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOptions...)

	ctxt, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(session.Printf))
	if options.Timeout != 0 {
		ctxt, cancel = context.WithTimeout(ctxt, options.Timeout)
	}
	cancelFunc := func() {
		cancel()
		allocCancel()
	}

	downloadPath, err := filepath.Abs(session.FilePrefix + session.Name + "/chrome")
	if err != nil {
		return ctxt, cancelFunc, "", err
	}

	err = os.MkdirAll(downloadPath, 0777)
	if err != nil {
		return ctxt, cancelFunc, "", fmt.Errorf("couldn't create directory: %v", downloadPath)
	}

	// configure to download behavior
	err = chromedp.Run(ctxt,
		chromedp.ActionFunc(func(ctxt context.Context) error {
			err := page.SetDownloadBehavior("allow").WithDownloadPath(downloadPath).Do(ctxt)
			if err != nil {
				return err
			}
			return nil
		}))

	if err != nil {
		return ctxt, cancelFunc, "", err
	}

	return ctxt, cancelFunc, downloadPath, nil
}

func (session *Session) NewChrome() (context.Context, context.CancelFunc, string, error) {
	return session.NewChromeOpt(NewChromeOptions{Headless: false})
}
