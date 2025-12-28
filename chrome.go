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

// File permission constants
const (
	DefaultFilePermission = 0644 // Read/write for owner, read for group and others
	DefaultDirPermission  = 0755 // Read/write/execute for owner, read/execute for group and others
)

// URL constants for replay mode
const (
	DefaultBlankURL = "about:blank"
)

type ChromeSession struct {
	*Session
	Ctx           context.Context
	DownloadPath  string
	lastHtml      string        // Last successfully retrieved HTML content
	ActionTimeout time.Duration // Timeout for individual actions (0 = no timeout)
}

// captureCurrentHtml safely captures the current HTML page content
// This is called before Chrome operations to preserve state for debugging timeouts
func (session *ChromeSession) captureCurrentHtml(ctx context.Context) {
	// Use defer/recover to safely handle any panics from chromedp operations
	defer func() {
		if r := recover(); r != nil {
			// Silently ignore panics - context might not have Chrome executor yet
		}
	}()

	// Use a short timeout to avoid blocking if HTML is not available yet
	// 2 seconds is enough for already-loaded pages (real usage pattern)
	// but won't block too long if page hasn't loaded yet (e.g., in tests)
	captureCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	var html string
	// Try to get HTML - this might panic if context doesn't have Chrome executor
	err := chromedp.OuterHTML("html", &html, chromedp.ByQuery).Do(captureCtx)
	if err == nil {
		session.lastHtml = html
	}
	// Silently ignore errors - HTML might not be available yet
}

// SaveLastHtmlSnapshot saves the last HTML content to a timestamped snapshot file
// This is useful for debugging timeouts - call this when an error occurs
// Multiple snapshots are preserved to track timeout sequences
func (session *ChromeSession) SaveLastHtmlSnapshot() error {
	if session.lastHtml == "" {
		return nil // Nothing to save
	}

	// Use timestamp to avoid overwriting previous snapshots
	timestamp := time.Now().Format("20060102-150405")
	filename := path.Join(session.getDirectory(), fmt.Sprintf("snapshot-%s.html", timestamp))

	err := os.WriteFile(filename, []byte(session.lastHtml), DefaultFilePermission)
	if err != nil {
		return fmt.Errorf("failed to save snapshot: %w", err)
	}

	session.Printf("Saved last HTML snapshot to %s\n", filename)
	return nil
}

// ActionCtx returns a context with ActionTimeout applied if set.
// Use this for individual chromedp actions that should respect the action timeout.
// Caller must call the returned cancel function when done.
//
// Example:
//
//	actionCtx, cancel := chromeSession.ActionCtx()
//	defer cancel()
//	err := chromedp.Run(actionCtx, chromedp.WaitVisible(selector, chromedp.ByQuery))
func (session *ChromeSession) ActionCtx() (context.Context, context.CancelFunc) {
	if session.ActionTimeout > 0 {
		return context.WithTimeout(session.Ctx, session.ActionTimeout)
	}
	return session.Ctx, func() {}
}

type NewChromeOptions struct {
	Headless          bool
	Timeout           time.Duration
	ActionTimeout     time.Duration // Timeout for individual actions (0 = no timeout)
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

	err = os.MkdirAll(downloadPath, DefaultDirPermission)
	if err != nil {
		return nil, func() {}, fmt.Errorf("couldn't create directory: %v", downloadPath)
	}

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOptions...)

	ctxt, cancel := chromedp.NewContext(allocCtx, chromedp.WithLogf(session.Printf))
	if options.Timeout != 0 {
		ctxt, cancel = context.WithTimeout(ctxt, options.Timeout)
	}

	chromeSession = &ChromeSession{
		Session:       session,
		Ctx:           ctxt,
		DownloadPath:  downloadPath,
		lastHtml:      "", // Initialize with empty string
		ActionTimeout: options.ActionTimeout,
	}

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

// loadSavedHTMLToBrowser loads saved HTML content into the browser for replay mode
func (session *ChromeSession) loadSavedHTMLToBrowser(filename string) error {
	// Read the saved HTML file
	htmlBytes, err := os.ReadFile(filename)
	if err != nil {
		return RetryAndRecordError{filename}
	}

	html := string(htmlBytes)

	// Try to load metadata for URL information
	metadata, err := loadPageMetadata(filename)
	if err != nil {
		// If metadata is not available, just load the HTML without URL context
		session.Printf("%s WARNING: failed to load metadata for %s: %v", session.getDebugPrefix(), filename, err)
		metadata = PageMetadata{URL: DefaultBlankURL}
	}

	// Load HTML into browser by writing to a temp file and loading it
	// This avoids URL encoding issues that occur with data: URIs
	tempFile := filename + ".temp.html"
	err = os.WriteFile(tempFile, htmlBytes, DefaultFilePermission)
	if err != nil {
		return fmt.Errorf("failed to create temp HTML file: %w", err)
	}
	defer os.Remove(tempFile)

	// Get absolute path for the temp file
	absPath, err := filepath.Abs(tempFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path for temp file %s: %w", tempFile, err)
	}

	// Load the temp file using file:// protocol with absolute path
	fileURL := "file://" + absPath
	err = chromedp.Run(session.Ctx,
		chromedp.Navigate(fileURL),
		// Wait for the DOM to be ready after navigation
		chromedp.WaitReady("body", chromedp.ByQuery))
	if err != nil {
		return fmt.Errorf("failed to load HTML into browser: %w", err)
	}

	// Set the URL context in browser if we have valid metadata
	if metadata.URL != "" && metadata.URL != DefaultBlankURL {
		// Use JavaScript to update the browser's location context for debugging
		// Note: This won't actually change the URL bar but helps with relative URL resolution
		script := fmt.Sprintf(`
			// Store original URL in a global variable for reference
			window.__replayOriginalURL = %q;
			// Update document.baseURI if possible (read-only in most browsers)
		`, metadata.URL)
		_ = chromedp.Run(session.Ctx, chromedp.Evaluate(script, nil))
	}

	session.Printf("%s REPLAY LOADED: %s (%d bytes)", session.getDebugPrefix(), filename, len(html))
	return nil
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
			// Replay mode: load from saved file and load it into the browser
			session.Printf("%s LOAD from %v\n", session.getDebugPrefix(), fn)

			// Load the saved HTML into the browser so DOM operations work
			err := session.loadSavedHTMLToBrowser(fn)
			if err != nil {
				return err
			}

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
			err = os.WriteFile(fn, body, DefaultFilePermission)
			if err != nil {
				return err
			}
			session.Printf("%s SAVE to %v (%v bytes)\n", session.getDebugPrefix(), fn, len(body))

			// Save metadata for replay mode
			var currentURL string
			var title string
			err = chromedp.Evaluate(`window.location.href`, &currentURL).Do(ctxt)
			if err != nil {
				session.Printf("Warning: failed to get current URL: %v", err)
			}

			err = chromedp.Title(&title).Do(ctxt)
			if err != nil {
				session.Printf("Warning: failed to get title: %v", err)
			} else {
				session.Printf("* %v\n", title)
			}

			// Save unified metadata
			metadata := PageMetadata{
				URL:         currentURL,
				ContentType: "text/html", // Chrome pages are typically HTML
				Title:       title,
			}
			err = savePageMetadata(fn, metadata)
			if err != nil {
				session.Printf("Warning: failed to save metadata: %v", err)
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
			return fmt.Errorf("filename parameter cannot be nil in DownloadFile")
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

			// No matching file found in replay mode
			return RetryAndRecordError{fmt.Sprintf("no files matching pattern '%s' found in download directory '%s' for replay mode", options.Glob, session.DownloadPath)}
		} else {
			// Record mode: perform actual download
			download := make(chan string)

			if options.Timeout == 0 {
				options.Timeout = 5 * time.Second
			}

			downloadCtx, cancel := context.WithTimeout(ctxt, options.Timeout)
			defer cancel()

			startTime := time.Now()

			// Capture current HTML before setting up download listener (for debugging timeouts)
			// This captures the page state before clicking download button
			session.captureCurrentHtml(ctxt)

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
			return fmt.Errorf("filename parameter cannot be nil in SaveFile")
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
			err = os.WriteFile(fn, body, DefaultFilePermission)
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
			err = os.WriteFile(responseFilename, jsonData, DefaultFilePermission)
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

// GetCurrentURL returns the current page URL
func (session *ChromeSession) GetCurrentURL() (string, error) {
	if session.NotUseNetwork {
		// Replay mode: load URL from saved metadata file
		// Use current invokeCount to get the right filename
		fn := fmt.Sprintf("%v/%v.html", session.getDirectory(), session.invokeCount)

		// Load unified metadata file
		metadata, err := loadPageMetadata(fn)
		if err != nil {
			return "", fmt.Errorf("failed to load metadata: %v", err)
		}
		if session.ShowResponseHeader {
			session.Printf("Current URL (replay): %s", metadata.URL)
		}
		return metadata.URL, nil
	} else {
		// Live mode: get URL from browser
		var currentURL string
		err := chromedp.Run(session.Ctx,
			chromedp.Evaluate(`window.location.href`, &currentURL),
		)
		if err != nil {
			return "", fmt.Errorf("failed to get current URL: %v", err)
		}
		if session.ShowResponseHeader {
			session.Printf("Current URL: %s", currentURL)
		}
		return currentURL, nil
	}
}

// UnifiedScraper interface implementation for ChromeSession

// Navigate returns an ActionFunc that handles navigation with replay support
func (chromeSession *ChromeSession) Navigate(url string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		if chromeSession.NotUseNetwork {
			// Replay mode: increment counter and load saved HTML
			// This simulates navigation by loading the next saved page
			return chromeSession.SaveHtml(nil).Do(ctx)
		} else {
			// Record mode: capture current HTML before navigation (for debugging timeouts)
			chromeSession.captureCurrentHtml(ctx)

			// Perform actual navigation
			return chromedp.Run(ctx, chromedp.Navigate(url), chromeSession.SaveHtml(nil))
		}
	}
}

// WaitVisible returns an ActionFunc that handles wait visible with replay support
func (chromeSession *ChromeSession) WaitVisible(selector string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		if chromeSession.NotUseNetwork {
			// Replay mode: load saved HTML first, then check if element is visible
			err := chromeSession.SaveHtml(nil).Do(ctx)
			if err != nil {
				return err
			}

			// Now verify the element exists in the loaded HTML using a simple existence check
			// In replay mode, we just need to verify the element exists rather than waiting for visibility
			var result bool
			err = chromedp.Run(ctx,
				chromedp.Evaluate(fmt.Sprintf("document.querySelector(%q) !== null", selector), &result))
			if err != nil {
				return fmt.Errorf("failed to check element visibility in replay mode: %w", err)
			}
			if !result {
				return fmt.Errorf("element %q not visible in replay mode", selector)
			}
			return nil
		} else {
			// Record mode: capture current HTML before waiting (for debugging timeouts)
			chromeSession.captureCurrentHtml(ctx)

			// Perform actual wait
			return chromedp.Run(ctx, chromedp.WaitVisible(selector, chromedp.ByQuery), chromeSession.SaveHtml(nil))
		}
	}
}

// SendKeys returns an ActionFunc that handles sending keys with replay support
func (chromeSession *ChromeSession) SendKeys(selector, value string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		if chromeSession.NotUseNetwork {
			// Replay mode: simulate action by loading next saved page
			return chromeSession.SaveHtml(nil).Do(ctx)
		} else {
			// Record mode: capture current HTML before action (for debugging timeouts)
			chromeSession.captureCurrentHtml(ctx)

			// Perform actual key sending
			return chromedp.Run(ctx, chromedp.SendKeys(selector, value, chromedp.ByQuery), chromeSession.SaveHtml(nil))
		}
	}
}

// Click returns an ActionFunc that handles clicking with replay support
func (chromeSession *ChromeSession) Click(selector string) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		if chromeSession.NotUseNetwork {
			// Replay mode: simulate click action by loading next saved page
			return chromeSession.SaveHtml(nil).Do(ctx)
		} else {
			// Record mode: capture current HTML before action (for debugging timeouts)
			chromeSession.captureCurrentHtml(ctx)

			// Perform actual click
			return chromedp.Run(ctx, chromedp.Click(selector, chromedp.ByQuery), chromeSession.SaveHtml(nil))
		}
	}
}

// SubmitForm implements UnifiedScraper.SubmitForm
func (chromeSession *ChromeSession) SubmitForm(formSelector string, params map[string]string) error {
	if chromeSession.NotUseNetwork {
		// Replay mode: simulate form submission by loading next saved page
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
		// Replay mode: simulate anchor following by loading next saved page
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

// Sleep returns an ActionFunc that handles sleeping with replay support.
// In replay mode, this method returns immediately without actual waiting, which speeds up test execution.
// In record mode, this method performs the actual sleep operation using chromedp.Sleep.
func (chromeSession *ChromeSession) Sleep(duration time.Duration) chromedp.ActionFunc {
	return func(ctx context.Context) error {
		if chromeSession.NotUseNetwork {
			// Replay mode: don't wait real time, just log
			chromeSession.Printf("%s REPLAY SLEEP: skipping %v\n", chromeSession.getDebugPrefix(), duration)
			return nil
		} else {
			// Record mode: perform actual sleep
			chromeSession.Printf("%s SLEEP: %v\n", chromeSession.getDebugPrefix(), duration)
			return chromedp.Run(ctx, chromedp.Sleep(duration))
		}
	}
}

// DoNavigate implements UnifiedScraper.Navigate
func (chromeSession *ChromeSession) DoNavigate(url string) error {
	return chromeSession.Navigate(url).Do(chromeSession.Ctx)
}

// DoWaitVisible implements UnifiedScraper.WaitVisible
func (chromeSession *ChromeSession) DoWaitVisible(selector string) error {
	return chromeSession.WaitVisible(selector).Do(chromeSession.Ctx)
}

// DoSendKeys implements UnifiedScraper.SendKeys
func (chromeSession *ChromeSession) DoSendKeys(selector, value string) error {
	return chromeSession.SendKeys(selector, value).Do(chromeSession.Ctx)
}

// DoClick implements UnifiedScraper.Click
func (chromeSession *ChromeSession) DoClick(selector string) error {
	return chromeSession.Click(selector).Do(chromeSession.Ctx)
}

// DoSleep is a convenience method for executing Sleep action
func (chromeSession *ChromeSession) DoSleep(duration time.Duration) error {
	return chromeSession.Sleep(duration).Do(chromeSession.Ctx)
}
