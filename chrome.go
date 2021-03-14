package scraper

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"os"
	"path/filepath"
)

func (session *Session) NewChrome() (context.Context, context.CancelFunc, string, error) {
	chromeUserDataDir, err := filepath.Abs("./chromeUserData")
	if err != nil {
		return nil, func() {}, "", err
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(
		context.Background(),
		chromedp.UserDataDir(chromeUserDataDir),
	// chromedp.Headless,// headlessにするにはこの2行を有効にするのだが、ちゃんと終わらない...
	// chromedp.DisableGPU,
	)

	ctxt, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(session.Printf))
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
