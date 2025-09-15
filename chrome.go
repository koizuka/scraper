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
	Headless          bool
	Timeout           time.Duration
	ExtraAllocOptions []chromedp.ExecAllocatorOption
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

	// Add any extra allocator options
	if len(options.ExtraAllocOptions) > 0 {
		allocOptions = append(allocOptions, options.ExtraAllocOptions...)
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
		session.invokeCount++
		fn := session.getHtmlFilename()

		if filename != nil {
			*filename = fn
		}

		if session.NotUseNetwork {
			// Replay mode: load from saved file
			session.Printf("%s LOAD from %v\n", session.getDebugPrefix(), fn)
			body, err := os.ReadFile(fn)
			if err != nil {
				return RetryAndRecordError{fn}
			}
			session.Printf("%s LOADED %v (%v bytes)\n", session.getDebugPrefix(), fn, len(body))
			return nil
		} else {
			// Record mode: get HTML from browser and save
			var html string
			err := chromedp.OuterHTML("html", &html, chromedp.ByQuery).Do(ctxt)
			if err != nil {
				return err
			}

			body := []byte(html)

			// Always save HTML (backward compatibility - SaveHtml traditionally always saves)
			err = os.WriteFile(fn, body, 0644)
			if err != nil {
				return err
			}
			session.Printf("%s SAVE to %v (%v bytes)\n", session.getDebugPrefix(), fn, len(body))

			var title string
			err = chromedp.Title(&title).Do(ctxt)
			if err == nil {
				session.Printf("* %v\n", title)
			}
			return nil
		}
	})
}

type DownloadFileOptions struct {
	Timeout time.Duration
	Glob    string
}

type DownloadedFileNameNotSatisfiedError struct {
	DownloadedFilename string
	Glob               string
}

func (e *DownloadedFileNameNotSatisfiedError) Error() string {
	return fmt.Sprintf("downloaded filename %v is not match to %v", e.DownloadedFilename, e.Glob)
}

func (session *ChromeSession) DownloadFile(filename *string, options DownloadFileOptions, actions ...chromedp.Action) chromedp.ActionFunc {
	return func(ctxt context.Context) error {
		if filename == nil {
			return fmt.Errorf("filename is nil")
		}

		if options.Glob == "" {
			options.Glob = "*"
		} else {
			// if options.Glob has path separator, it's invalid
			if dir, _ := path.Split(options.Glob); dir != "" {
				return fmt.Errorf("invalid glob pattern(contains path component): %v", options.Glob)
			}
			// validate options.Glob
			_, err := filepath.Match(options.Glob, "")
			if err != nil {
				return fmt.Errorf("invalid glob pattern(%w): %v", err, options.Glob)
			}
		}

		if session.NotUseNetwork {
			// Replay mode: find previously downloaded file
			session.Printf("%s REPLAY DOWNLOAD: Looking for files matching %v\n", session.getDebugPrefix(), options.Glob)
			files, err := os.ReadDir(session.DownloadPath)
			if err != nil {
				return RetryAndRecordError{session.DownloadPath}
			}

			// Find the first file that matches the glob pattern
			for _, file := range files {
				if match, _ := filepath.Match(options.Glob, file.Name()); match {
					*filename = path.Join(session.DownloadPath, file.Name())
					session.Printf("%s REPLAY DOWNLOADED: %v\n", session.getDebugPrefix(), *filename)
					return nil
				}
			}

			// No matching file found
			return RetryAndRecordError{fmt.Sprintf("%s/%s", session.DownloadPath, options.Glob)}
		} else {
			// Record mode: perform actual download
			download := make(chan string)

			if options.Timeout == 0 {
				options.Timeout = 5 * time.Second
			}

			downloadCtx, cancel := context.WithTimeout(ctxt, options.Timeout)
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
				var matchErr error
				latestTime := time.Time{}
				for _, file := range files {
					info, err := file.Info()
					if err != nil {
						return err
					}
					if !info.ModTime().Before(startTime) {
						if match, _ := filepath.Match(options.Glob, file.Name()); match {
							*filename = path.Join(session.DownloadPath, file.Name())
							session.Printf("%s DOWNLOADED: %v\n", session.getDebugPrefix(), *filename)
							return nil
						} else {
							if info.ModTime().After(latestTime) {
								latestTime = info.ModTime()
								matchErr = &DownloadedFileNameNotSatisfiedError{DownloadedFilename: file.Name(), Glob: options.Glob}
							}
						}
					}
				}
				if matchErr != nil {
					return matchErr
				}
				return downloadCtx.Err()

			case downloaded := <-download:
				if downloaded == "" {
					return fmt.Errorf("download canceled")
				}
				// if downloaded filename is not match to options.Glob pattern, error
				if match, _ := filepath.Match(options.Glob, filepath.Base(downloaded)); !match {
					return &DownloadedFileNameNotSatisfiedError{DownloadedFilename: downloaded, Glob: options.Glob}
				}

				*filename = downloaded
				session.Printf("%s DOWNLOADED: %v\n", session.getDebugPrefix(), *filename)
				return nil
			}
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

		if session.NotUseNetwork {
			// Replay mode: check if saved file exists
			if _, err := os.Stat(fn); os.IsNotExist(err) {
				return RetryAndRecordError{fn}
			}
			session.Printf("%s REPLAY SAVE: file already exists %v\n", session.getDebugPrefix(), fn)
			return nil
		} else {
			// Record mode: perform actual file save
			body, err := os.ReadFile(*filename)
			if err != nil {
				return err
			}
			err = os.WriteFile(fn, body, 0644)
			if err != nil {
				return err
			}
			session.Printf("%s SAVE %v to %v (%v bytes)\n", session.getDebugPrefix(), *filename, fn, len(body))
			return nil
		}
	}
}

func (session *ChromeSession) actionChrome(action chromedp.Action) (*network.Response, error) {
	var filename string

	if session.NotUseNetwork {
		// Replay mode: just call SaveHtml to increment counter and load saved HTML
		err := session.SaveHtml(&filename).Do(session.Ctx)
		if err != nil {
			return nil, err
		}

		// Try to load response JSON if it exists
		responseFilename := filename + ".response.json"
		if jsonData, err := os.ReadFile(responseFilename); err == nil {
			var resp network.Response
			if json.Unmarshal(jsonData, &resp) == nil {
				return &resp, nil
			}
		}

		// Return dummy response if JSON not found
		return &network.Response{Status: 200}, nil
	} else {
		// Record mode: perform actual action
		resp, err := chromedp.RunResponse(session.Ctx, chromedp.Tasks{
			action,
			session.SaveHtml(&filename),
		})
		if err != nil {
			return nil, err
		}

		// Always save response JSON (backward compatibility)
		responseFilename := filename + ".response.json"
		jsonData, err := json.Marshal(resp)
		if err == nil {
			err = os.WriteFile(responseFilename, jsonData, 0644)
			if err != nil {
				return nil, err
			}
		}

		return resp, nil
	}
}

// RunNavigate navigates to page URL and download html like Session.invoke
func (session *ChromeSession) RunNavigate(URL string) (*network.Response, error) {
	return session.actionChrome(chromedp.Navigate(URL))
}

func (session *ChromeSession) Unmarshal(v interface{}, cssSelector string, opt UnmarshalOption) error {
	return ChromeUnmarshal(session.Ctx, v, cssSelector, opt)
}

// UnifiedScraper interface implementation for ChromeSession

// Navigate implements UnifiedScraper.Navigate
func (chromeSession *ChromeSession) Navigate(url string) error {
	if chromeSession.NotUseNetwork {
		// Replay mode: just call SaveHtml to increment counter
		return chromeSession.SaveHtml(nil).Do(chromeSession.Ctx)
	} else {
		// Record mode: perform actual navigation
		return chromedp.Run(chromeSession.Ctx, chromedp.Navigate(url), chromeSession.SaveHtml(nil))
	}
}

// WaitVisible implements UnifiedScraper.WaitVisible
func (chromeSession *ChromeSession) WaitVisible(selector string) error {
	if chromeSession.NotUseNetwork {
		// Replay mode: just call SaveHtml to increment counter
		return chromeSession.SaveHtml(nil).Do(chromeSession.Ctx)
	} else {
		// Record mode: perform actual wait
		return chromedp.Run(chromeSession.Ctx, chromedp.WaitVisible(selector, chromedp.ByQuery), chromeSession.SaveHtml(nil))
	}
}

// SendKeys implements UnifiedScraper.SendKeys
func (chromeSession *ChromeSession) SendKeys(selector, value string) error {
	if chromeSession.NotUseNetwork {
		// Replay mode: just call SaveHtml to increment counter
		return chromeSession.SaveHtml(nil).Do(chromeSession.Ctx)
	} else {
		// Record mode: perform actual key sending
		return chromedp.Run(chromeSession.Ctx, chromedp.SendKeys(selector, value, chromedp.ByQuery), chromeSession.SaveHtml(nil))
	}
}

// Click implements UnifiedScraper.Click
func (chromeSession *ChromeSession) Click(selector string) error {
	if chromeSession.NotUseNetwork {
		// Replay mode: just call SaveHtml to increment counter
		return chromeSession.SaveHtml(nil).Do(chromeSession.Ctx)
	} else {
		// Record mode: perform actual click
		return chromedp.Run(chromeSession.Ctx, chromedp.Click(selector, chromedp.ByQuery), chromeSession.SaveHtml(nil))
	}
}

// SubmitForm implements UnifiedScraper.SubmitForm
func (chromeSession *ChromeSession) SubmitForm(formSelector string, params map[string]string) error {
	if chromeSession.NotUseNetwork {
		// Replay mode: just call SaveHtml to increment counter
		return chromeSession.SaveHtml(nil).Do(chromeSession.Ctx)
	} else {
		// Record mode: perform actual form submission
		var tasks []chromedp.Action

		// Fill form fields - clear first, then send keys
		for selector, value := range params {
			tasks = append(tasks,
				chromedp.Clear(selector, chromedp.ByQuery),
				chromedp.SendKeys(selector, value, chromedp.ByQuery),
			)
		}

		// Try multiple submit strategies
		submitSelectors := []string{
			formSelector + " input[type=submit]",
			formSelector + " button[type=submit]",
			formSelector + " button:not([type])", // default button type is submit
			formSelector + " input[type=image]",  // image submit buttons
		}

		// Try each selector until one works
		var lastErr error
		for _, submitSelector := range submitSelectors {
			submitTasks := append(tasks, chromedp.Click(submitSelector, chromedp.ByQuery), chromeSession.SaveHtml(nil))
			err := chromedp.Run(chromeSession.Ctx, submitTasks...)
			if err == nil {
				return nil
			}
			lastErr = err
			// Continue to next selector if this one failed
		}

		// If all submit button attempts failed, try submitting via Enter key on form
		if len(params) > 0 {
			// Get the last field selector and try pressing Enter
			for selector := range params {
				enterTasks := append(tasks, chromedp.SendKeys(selector, "\n", chromedp.ByQuery), chromeSession.SaveHtml(nil))
				err := chromedp.Run(chromeSession.Ctx, enterTasks...)
				if err == nil {
					return nil
				}
				break // Only try the first field
			}
		}

		return fmt.Errorf("failed to submit form %q: %w", formSelector, lastErr)
	}
}

// FollowAnchor implements UnifiedScraper.FollowAnchor
func (chromeSession *ChromeSession) FollowAnchor(text string) error {
	if chromeSession.NotUseNetwork {
		// Replay mode: just call SaveHtml to increment counter
		return chromeSession.SaveHtml(nil).Do(chromeSession.Ctx)
	} else {
		// Record mode: perform actual anchor follow
		// Use proper XPath escaping to prevent injection attacks
		// XPath 1.0 doesn't have a built-in escape function, so we construct a concat() expression
		escapedText := escapeXPathText(text)
		xpath := fmt.Sprintf("//a[contains(text(), %s)]", escapedText)
		return chromedp.Run(chromeSession.Ctx, chromedp.Click(xpath, chromedp.BySearch), chromeSession.SaveHtml(nil))
	}
}

// escapeXPathText properly escapes text for use in XPath expressions
// Uses concat() to handle both single and double quotes safely
func escapeXPathText(text string) string {
	// If text contains no quotes, wrap in single quotes
	if !strings.Contains(text, "'") && !strings.Contains(text, "\"") {
		return "'" + text + "'"
	}

	// If text contains only double quotes (no single quotes), wrap in single quotes
	if !strings.Contains(text, "'") {
		return "'" + text + "'"
	}

	// If text contains only single quotes (no double quotes), wrap in double quotes
	if !strings.Contains(text, "\"") {
		return "\"" + text + "\""
	}

	// If text contains both single and double quotes, use concat()
	parts := strings.Split(text, "'")
	var concatParts []string

	for i, part := range parts {
		if i > 0 {
			// Add the single quote as a separate part
			concatParts = append(concatParts, "\"'\"")
		}
		if part != "" {
			concatParts = append(concatParts, "'"+part+"'")
		}
	}

	if len(concatParts) == 1 {
		return concatParts[0]
	}

	return "concat(" + strings.Join(concatParts, ", ") + ")"
}

// SavePage implements UnifiedScraper.SavePage (calls existing SaveHtml action)
func (chromeSession *ChromeSession) SavePage() (string, error) {
	var filename string
	action := chromeSession.SaveHtml(&filename)
	err := chromedp.Run(chromeSession.Ctx, action)
	return filename, err
}

// ExtractData implements UnifiedScraper.ExtractData
func (chromeSession *ChromeSession) ExtractData(v interface{}, selector string, opt UnmarshalOption) error {
	return ChromeUnmarshal(chromeSession.Ctx, v, selector, opt)
}

// DownloadResource implements UnifiedScraper.DownloadResource
func (chromeSession *ChromeSession) DownloadResource(options UnifiedDownloadOptions) (string, error) {
	// Set default timeout if not specified
	timeout := options.Timeout
	if timeout == 0 {
		timeout = DefaultDownloadTimeout
	}

	var filename string
	downloadOptions := DownloadFileOptions{
		Timeout: timeout,
		Glob:    options.Glob,
	}

	// Create a basic download action that waits for any download to complete
	// This is a simplified implementation - for complex scenarios, use DownloadFile directly
	action := chromeSession.DownloadFile(&filename, downloadOptions)
	err := chromedp.Run(chromeSession.Ctx, action)

	if err != nil {
		return "", fmt.Errorf("download failed - consider using DownloadFile method directly for complex scenarios: %w", err)
	}

	return filename, nil
}

// GetDebugStep implements UnifiedScraper.GetDebugStep
func (chromeSession *ChromeSession) GetDebugStep() string {
	return chromeSession.Session.GetDebugStep()
}

// SetDebugStep implements UnifiedScraper.SetDebugStep
func (chromeSession *ChromeSession) SetDebugStep(step string) {
	chromeSession.Session.SetDebugStep(step)
}

// ClearDebugStep implements UnifiedScraper.ClearDebugStep
func (chromeSession *ChromeSession) ClearDebugStep() {
	chromeSession.Session.ClearDebugStep()
}

// Printf implements UnifiedScraper.Printf
func (chromeSession *ChromeSession) Printf(format string, a ...interface{}) {
	chromeSession.Session.Printf(format, a...)
}

// Sleep implements sleep functionality with replay mode support
func (chromeSession *ChromeSession) Sleep(duration time.Duration) error {
	if chromeSession.NotUseNetwork {
		// Replay mode: don't wait real time, just log
		chromeSession.Printf("%s REPLAY SLEEP: skipping %v\n", chromeSession.getDebugPrefix(), duration)
		return nil
	} else {
		// Record mode: perform actual sleep
		chromeSession.Printf("%s SLEEP: %v\n", chromeSession.getDebugPrefix(), duration)
		return chromedp.Run(chromeSession.Ctx, chromedp.Sleep(duration))
	}
}
