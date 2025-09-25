package scraper

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	cookiejar "github.com/orirawlings/persistent-cookiejar"
	"golang.org/x/text/encoding"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	//UserAgent_Chrome39  = "Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.99 Safari/537.36"
	//UserAgent_iOS8      = "Mozilla/5.0 (iPhone; CPU iPhone OS 8_1_3 like Mac OS X) AppleWebKit/600.1.4 (KHTML, like Gecko) Mobile/12B466"
	UserAgent_firefox86 = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:86.0) Gecko/20100101 Firefox/86.0"
	UserAgent_default   = UserAgent_firefox86
)

// Session holds communication and logging options
type Session struct {
	Name               string // directory name to store session files(downloaded files and cookies)
	client             http.Client
	Encoding           encoding.Encoding // force charset over Content-Type response header
	UserAgent          string            // specify User-Agent
	FilePrefix         string            // prefix to directory of session files
	invokeCount        int
	NotUseNetwork      bool // load from previously downloaded session files rather than network access
	SaveToFile         bool // save downloaded pages to session directory
	ShowRequestHeader  bool // print request headers with Logger
	ShowResponseHeader bool // print response headers with Logger
	ShowFormPosting    bool // print posting form data, with Logger
	Log                Logger
	jar                *cookiejar.Jar
	BodyFilter         func(resp *Response, body []byte) ([]byte, error)
	debugStep          string // debug step label for logging

	// Fields for unified scraper interface
	currentPage     *Page             // Current page for unified operations
	pendingFormData map[string]string // Form data to be submitted
	mu              sync.RWMutex      // Mutex for thread safety
}

type RequestError struct {
	RequestURL *url.URL
	Err        error
}

func (err RequestError) Error() string {
	return fmt.Sprintf("%v request error: %v", err.RequestURL.String(), err.Err)
}

type ResponseError struct {
	RequestURL *url.URL
	Response   *http.Response
}

func (err ResponseError) Error() string {
	return fmt.Sprintf("%v response code: %v", err.RequestURL.String(), err.Response.Status)
}

func NewSession(name string, log Logger) *Session {
	jar, _ := cookiejar.New(nil)
	return &Session{
		Name:      name,
		UserAgent: UserAgent_default,
		client: http.Client{
			Jar: jar,
		},
		Log: log,
		jar: jar,
	}
}

func (session *Session) Printf(format string, a ...interface{}) {
	session.Log.Printf(format, a...)
}

func (session *Session) Cookies(u *url.URL) []*http.Cookie {
	return session.client.Jar.Cookies(u)
}

func (session *Session) SetCookies(u *url.URL, cookies []*http.Cookie) {
	session.client.Jar.SetCookies(u, cookies)
}

func (session *Session) LoadCookie() error {
	filename := fmt.Sprintf("%v/cookie", session.getDirectory())

	jar, err := cookiejar.New(&cookiejar.Options{
		Filename:              filename,
		PersistSessionCookies: true,
	})
	if err == nil {
		session.jar = jar
		session.client.Jar = jar
	}
	return err
}

// SaveCookie stores cookies to a file.
// must call LoadCookie() before call SaveCookie().
func (session *Session) SaveCookie() error {
	return session.jar.Save()
}

// SetDebugStep sets the debug step label for logging
func (session *Session) SetDebugStep(step string) {
	session.debugStep = step
	session.Printf("**** [%s] START\n", step)
}

// ClearDebugStep clears the debug step label
func (session *Session) ClearDebugStep() {
	if session.debugStep != "" {
		session.Printf("**** [%s] END\n", session.debugStep)
	}
	session.debugStep = ""
}

// getDebugPrefix returns the debug prefix for logging
func (session *Session) getDebugPrefix() string {
	if session.debugStep == "" {
		return "****"
	}
	return fmt.Sprintf("**** [%s]", session.debugStep)
}

func (session *Session) getDirectory() string {
	return fmt.Sprintf("%v%v", session.FilePrefix, session.Name)
}

func (session *Session) getHtmlFilename() string {
	return path.Join(session.getDirectory(), fmt.Sprintf("%v.html", session.invokeCount))
}

func (session *Session) invoke(req *http.Request) (*Response, error) {
	var body []byte
	var contentType string

	if session.NotUseNetwork || session.SaveToFile {
		dirname := session.getDirectory()
		if _, err := os.Stat(dirname); err != nil && os.IsNotExist(err) {
			if err := os.Mkdir(dirname, os.FileMode(0744)); err != nil {
				return nil, err
			}
		}
	}

	session.invokeCount++
	filename := session.getHtmlFilename()

	if session.ShowRequestHeader {
		session.Printf("REQUEST: %v %v:\n", req.Method, req.URL.String())
	}

	if !session.NotUseNetwork {
		userAgent := session.UserAgent
		if userAgent == "" {
			userAgent = UserAgent_default
		}
		req.Header.Set("User-agent", userAgent)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Upgrade-Insecure-Requests", "1")
		req.Header.Set("DNT", "1")

		if session.ShowRequestHeader {
			session.Printf("Request header:{\n")
			for k, v := range req.Header {
				session.Printf("  %v: %v\n", k, v)
			}
			session.Printf("}\n")

			//session.Printf("req = %v\n", req)
		}

		response, err := session.client.Do(req)
		if err != nil {
			return nil, RequestError{req.URL, err}
		}
		defer func() {
			_ = response.Body.Close()
		}()

		req = response.Request // update req.Url after redirects

		if response.StatusCode/100 != 2 {
			return nil, ResponseError{req.URL, response}
		}

		if session.ShowResponseHeader {
			session.Printf("Response Header:\n")
			for k, v := range response.Header {
				session.Printf("  %v: %v\n", k, v)
			}
		}

		contentType = response.Header.Get("content-type")

		body, err = io.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		if session.SaveToFile {
			// save to file
			session.Printf("%s SAVE to %v (%v bytes)\n", session.getDebugPrefix(), filename, len(body))
			err = os.WriteFile(filename, body, os.FileMode(0644))
			if err != nil {
				return nil, err
			}

			// Save metadata to unified file
			metadata := PageMetadata{
				URL:         req.URL.String(),
				ContentType: contentType,
			}
			err = savePageMetadata(filename, metadata)
			if err != nil {
				return nil, err
			}
		}
	} else {
		// load from file
		session.Printf("%s LOAD from %v\n", session.getDebugPrefix(), filename)
		var err error
		body, err = os.ReadFile(filename)
		if err != nil {
			return nil, RetryAndRecordError{filename}
		}

		// Load metadata from unified file
		metadata, err := loadPageMetadata(filename)
		if err != nil {
			return nil, RetryAndRecordError{filename}
		}
		contentType = metadata.ContentType

		// Parse the saved URL and update request URL for proper replay
		if savedURL, parseErr := url.Parse(metadata.URL); parseErr == nil {
			req.URL = savedURL
		}
	}

	if session.ShowResponseHeader {
		session.Printf("Content-type: %v\n", contentType)
	}

	return &Response{
		Request:     req,
		ContentType: contentType,
		RawBody:     body,
		Encoding:    session.Encoding,
		Logger:      session,
	}, nil
}

// Get invokes HTTP GET request.
func (session *Session) Get(getUrl string) (*Response, error) {
	req, err := http.NewRequest("GET", getUrl, nil)
	if err != nil {
		return nil, err
	}
	return session.invoke(req)
}

// GetPageMaxRedirect gets the URL and follows HTTP meta refresh if response page contained that.
func (session *Session) GetPageMaxRedirect(getUrl string, maxRedirect int) (*Page, error) {
	resp, err := session.Get(getUrl)
	if err != nil {
		return nil, err
	}
	page, err := resp.PageOpt(PageOption{BodyFilter: session.BodyFilter})
	if err != nil {
		return nil, err
	}
	return session.ApplyRefresh(page, maxRedirect)
}

// ApplyRefresh mimics HTML Meta Refresh.
func (session *Session) ApplyRefresh(page *Page, maxRedirect int) (*Page, error) {
	if maxRedirect > 0 {
		if newUrl := page.MetaRefresh(); newUrl != nil {
			session.Printf("HTML Meta Refresh to: %v\n", newUrl)
			return session.GetPageMaxRedirect(newUrl.String(), maxRedirect-1)
		}
	}
	session.mu.Lock()
	session.currentPage = page
	session.mu.Unlock()
	return page, nil
}

// GetPage gets the URL and returns a Page.
func (session *Session) GetPage(getUrl string) (*Page, error) {
	return session.GetPageMaxRedirect(getUrl, 1)
}

// GetCurrentURL returns the current page URL
func (session *Session) GetCurrentURL() (string, error) {
	session.mu.RLock()
	defer session.mu.RUnlock()

	if session.currentPage == nil || session.currentPage.BaseUrl == nil {
		return "", fmt.Errorf("no current page available")
	}

	currentURL := session.currentPage.BaseUrl.String()
	if session.ShowResponseHeader {
		session.Printf("Current URL: %s", currentURL)
	}
	return currentURL, nil
}

// FormAction submits a form (easy version)
func (session *Session) FormAction(page *Page, formSelector string, params map[string]string) (*Response, error) {
	form, err := page.Form(formSelector)
	if err != nil {
		return nil, err
	}

	for key, value := range params {
		err = form.Set(key, value)
		if err != nil {
			return nil, err
		}
	}

	return session.Submit(form)
}

// FollowSelectionLink opens a link specified by attr of the selection and returns a Response.
func (session *Session) FollowSelectionLink(page *Page, selection *goquery.Selection, attr string) (*Response, error) {
	numLink := selection.Length()
	if numLink != 1 {
		return nil, fmt.Errorf("%v: found %v items", page.Url.String(), numLink)
	}
	linkUrl, ok := selection.Attr(attr)
	if !ok {
		return nil, fmt.Errorf("'%v': missing %v", page.Url.String(), attr)
	}

	reqUrl, err := page.ResolveLink(linkUrl)
	if err != nil {
		return nil, err
	}
	return session.OpenURL(page, reqUrl)
}

// OpenURL invokes HTTP GET request with referer header as page's URL.
func (session *Session) OpenURL(page *Page, url string) (*Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Referer", page.Url.String())
	return session.invoke(req)
}

func (session *Session) FollowLink(page *Page, linkSelector string, attr string) (*Response, error) {
	selection := page.Find(linkSelector)
	numLink := selection.Length()
	if numLink != 1 {
		return nil, fmt.Errorf("%v '%v': found %v items", page.Url.String(), linkSelector, numLink)
	}
	if _, ok := selection.Attr(attr); !ok {
		return nil, fmt.Errorf("%v '%v': missing %v", page.Url.String(), linkSelector, attr)
	}

	return session.FollowSelectionLink(page, selection, attr)
}

// Frame returns a Page of specified frameSelector.
func (session *Session) Frame(page *Page, frameSelector string) (*Page, error) {
	resp, err := session.FollowLink(page, frameSelector, "src")
	if err != nil {
		return nil, err
	}
	return resp.PageOpt(PageOption{BodyFilter: session.BodyFilter})
}

type FollowAnchorTextOption struct {
	CheckAlt  bool // if true, searches text into img.alt attribute
	NumLink   int  // if >0, must be equal to number of matched texts
	Index     int  // 0=use the first match
	TrimSpace bool // TrimSpace both before compare texts
}

func (session *Session) FollowAnchorTextOpt(page *Page, text string, opt FollowAnchorTextOption) (*Response, error) {
	session.Printf("FollowAnchorText: searching %#v\n", text)
	if opt.TrimSpace {
		text = strings.TrimSpace(text)
	}
	selection := page.Find("a").FilterFunction(func(_ int, s *goquery.Selection) bool {
		t := s.Text()
		if opt.TrimSpace {
			t = strings.TrimSpace(t)
		}
		if t == text {
			return true
		}
		if opt.CheckAlt {
			img := s.Find("img").FilterFunction(func(_ int, img *goquery.Selection) bool {
				alt, ok := img.Attr("alt")
				return ok && alt == text
			})
			return img.Length() > 0
		}
		return false
	})
	numLink := selection.Length()
	if numLink != opt.NumLink && (numLink == 0 || opt.NumLink > 0) {
		return nil, fmt.Errorf("%v '%v': found %v items", page.Url.String(), text, numLink)
	}

	return session.FollowSelectionLink(page, selection.Eq(opt.Index), "href")
}

func (session *Session) FollowAnchorText(page *Page, text string) (*Response, error) {
	return session.FollowAnchorTextOpt(page, text,
		FollowAnchorTextOption{CheckAlt: true, NumLink: 1},
	)
}

// UnifiedScraper interface implementation for Session

// Navigate implements UnifiedScraper.Navigate
func (session *Session) Navigate(url string) error {
	page, err := session.GetPage(url)
	if err != nil {
		return err
	}
	session.currentPage = page
	return nil
}

// WaitVisible implements UnifiedScraper.WaitVisible
// For HTTP-based scraping, this is a no-op since elements are immediately available
func (session *Session) WaitVisible(selector string) error {
	return nil
}

// SendKeys implements UnifiedScraper.SendKeys
// For HTTP scraping, this stores form data to be submitted later
func (session *Session) SendKeys(selector, value string) error {
	session.mu.Lock()
	defer session.mu.Unlock()

	// Store form data in session for later submission
	if session.pendingFormData == nil {
		session.pendingFormData = make(map[string]string)
	}
	session.pendingFormData[selector] = value
	return nil
}

// Click implements UnifiedScraper.Click
func (session *Session) Click(selector string) error {
	session.mu.RLock()
	currentPage := session.currentPage
	session.mu.RUnlock()

	if currentPage == nil {
		return fmt.Errorf("session: no current page available for click")
	}

	// Try to find a clickable element (link or form submit)
	selection := currentPage.Find(selector)
	if selection.Length() == 0 {
		return fmt.Errorf("session: selector %q not found", selector)
	}

	// Check if it's a link
	if _, exists := selection.Attr("href"); exists {
		resp, err := session.FollowSelectionLink(currentPage, selection, "href")
		if err != nil {
			return err
		}
		page, err := resp.Page()
		if err != nil {
			return err
		}
		session.mu.Lock()
		session.currentPage = page
		session.mu.Unlock()
		return nil
	}

	// Check if it's a form submit button
	if selection.Is("input[type=submit], button[type=submit], button") {
		form := selection.Closest("form")
		if form.Length() > 0 {
			formSelector := ""
			if name, exists := form.Attr("name"); exists {
				formSelector = fmt.Sprintf("form[name=%s]", name)
			} else if id, exists := form.Attr("id"); exists {
				formSelector = fmt.Sprintf("form#%s", id)
			} else {
				formSelector = "form"
			}

			session.mu.RLock()
			formData := make(map[string]string)
			for k, v := range session.pendingFormData {
				formData[k] = v
			}
			session.mu.RUnlock()
			return session.submitUnified(formSelector, formData)
		}
	}

	return fmt.Errorf("session: selector %q is not clickable", selector)
}

// SubmitForm implements UnifiedScraper.SubmitForm
func (session *Session) SubmitForm(formSelector string, params map[string]string) error {
	return session.submitUnified(formSelector, params)
}

// submitUnified handles form submission for the unified interface
func (session *Session) submitUnified(formSelector string, params map[string]string) error {
	session.mu.RLock()
	currentPage := session.currentPage
	session.mu.RUnlock()

	if currentPage == nil {
		return fmt.Errorf("session: no current page available for form submission")
	}

	// Ensure cleanup of pending form data on exit (defer handles all exit paths)
	defer func() {
		session.mu.Lock()
		session.pendingFormData = make(map[string]string) // Clear to prevent memory leaks
		session.mu.Unlock()
	}()

	if params == nil {
		session.mu.RLock()
		params = make(map[string]string)
		for k, v := range session.pendingFormData {
			params[k] = v
		}
		session.mu.RUnlock()
	}

	// Convert CSS selectors to form field names for HTTP scraping
	// For input[name=username] -> username
	convertedParams := make(map[string]string)
	for selector, value := range params {
		fieldName := extractNameFromSelector(selector)
		if fieldName != "" {
			convertedParams[fieldName] = value
		} else {
			// If we can't extract the name, use the selector as-is
			convertedParams[selector] = value
		}
	}

	resp, err := session.FormAction(currentPage, formSelector, convertedParams)
	if err != nil {
		return err
	}

	page, err := resp.Page()
	if err != nil {
		return err
	}

	session.mu.Lock()
	session.currentPage = page
	session.mu.Unlock()

	return nil
}

// FollowAnchor implements UnifiedScraper.FollowAnchor
func (session *Session) FollowAnchor(text string) error {
	session.mu.RLock()
	currentPage := session.currentPage
	session.mu.RUnlock()

	if currentPage == nil {
		return fmt.Errorf("session: no current page available for link following")
	}

	resp, err := session.FollowAnchorText(currentPage, text)
	if err != nil {
		return err
	}

	page, err := resp.Page()
	if err != nil {
		return err
	}

	session.mu.Lock()
	session.currentPage = page
	session.mu.Unlock()
	return nil
}

// SavePage implements UnifiedScraper.SavePage
func (session *Session) SavePage() (string, error) {
	if session.currentPage == nil {
		return "", fmt.Errorf("session: no current page available")
	}

	// Ensure directory exists
	dirname := session.getDirectory()
	if _, err := os.Stat(dirname); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(dirname, os.FileMode(0755)); err != nil {
			return "", fmt.Errorf("session: failed to create directory %s: %w", dirname, err)
		}
	}

	session.invokeCount++
	filename := session.getHtmlFilename()

	html, err := session.currentPage.Html()
	if err != nil {
		return "", err
	}

	body := []byte(html)
	err = os.WriteFile(filename, body, 0644)
	if err != nil {
		return "", err
	}

	session.Printf("%s SAVE to %v (%v bytes)\n", session.getDebugPrefix(), filename, len(body))
	return filename, nil
}

// ExtractData implements UnifiedScraper.ExtractData
func (session *Session) ExtractData(v interface{}, selector string, opt UnmarshalOption) error {
	if session.currentPage == nil {
		return fmt.Errorf("session: no current page available for data extraction")
	}

	selection := session.currentPage.Find(selector)
	return Unmarshal(v, selection, opt)
}

// DownloadResource implements UnifiedScraper.DownloadResource
func (session *Session) DownloadResource(options UnifiedDownloadOptions) (string, error) {
	// For HTTP scraping, we need to have already followed a download link
	// This method assumes the current response is a downloadable file
	if session.currentPage == nil {
		return "", fmt.Errorf("session: no current page available for download")
	}

	// For HTTP downloads, we can try to save the current response if it's a file
	// This is a basic implementation - more complex scenarios should use direct HTTP methods
	session.invokeCount++
	filename := session.getHtmlFilename()

	// Change extension based on content type or SaveAs option
	if options.SaveAs != "" {
		filename = path.Join(session.getDirectory(), options.SaveAs)
	}

	// Try to get the raw content (this is a simplified approach)
	html, err := session.currentPage.Html()
	if err != nil {
		return "", fmt.Errorf("session: failed to get page content for download: %w", err)
	}

	body := []byte(html)
	err = os.WriteFile(filename, body, 0644)
	if err != nil {
		return "", fmt.Errorf("session: failed to save downloaded content: %w", err)
	}

	session.Printf("%s DOWNLOAD saved to %v (%v bytes)\n", session.getDebugPrefix(), filename, len(body))
	return filename, nil
}

// GetDebugStep implements UnifiedScraper.GetDebugStep
func (session *Session) GetDebugStep() string {
	return session.debugStep
}

// extractNameFromSelector extracts the name attribute from CSS selectors
// Examples: input[name=username] -> username, [name="password"] -> password
func extractNameFromSelector(selector string) string {
	// Match patterns like input[name=fieldname] or [name="fieldname"]
	re := regexp.MustCompile(`\[name=["']?([^"'\]]+)["']?\]`)
	matches := re.FindStringSubmatch(selector)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// SetDebugStep implements UnifiedScraper.SetDebugStep - already exists

// ClearDebugStep implements UnifiedScraper.ClearDebugStep - already exists

// UnifiedScraper interface implementation

// Run executes a sequence of UnifiedActions
func (session *Session) Run(actions ...UnifiedAction) error {
	for _, action := range actions {
		if err := action.Do(session); err != nil {
			return err
		}
	}
	return nil
}

// IsReplayMode returns true if the scraper is in replay mode
func (session *Session) IsReplayMode() bool {
	return session.NotUseNetwork
}

// Action method implementations for UnifiedScraper

// navigateAction performs navigation
func (session *Session) navigateAction(url string) error {
	page, err := session.GetPage(url)
	if err != nil {
		return err
	}
	session.mu.Lock()
	session.currentPage = page
	session.mu.Unlock()
	return nil
}

// waitVisibleAction - no-op for HTTP scraping
func (session *Session) waitVisibleAction(selector string) error {
	// For HTTP-based scraping, this is a no-op since elements are immediately available
	return nil
}

// sendKeysAction stores form data to be submitted later
func (session *Session) sendKeysAction(selector, value string) error {
	session.mu.Lock()
	defer session.mu.Unlock()

	// Store form data in session for later submission
	if session.pendingFormData == nil {
		session.pendingFormData = make(map[string]string)
	}
	session.pendingFormData[selector] = value
	return nil
}

// clickAction performs click operation (delegates to Click method)
func (session *Session) clickAction(selector string) error {
	return session.Click(selector)
}

// sleepAction performs sleep (no-op in replay mode)
func (session *Session) sleepAction(duration time.Duration) error {
	if session.IsReplayMode() {
		// In replay mode, skip sleep and just log
		session.Printf("REPLAY SLEEP: skipping %v", duration)
		return nil
	}
	// For HTTP scraping, sleep is not typically needed, but we can do a simple time.Sleep
	session.Printf("SLEEP: %v", duration)
	time.Sleep(duration)
	return nil
}

// savePageAction saves the current page HTML
func (session *Session) savePageAction() error {
	if session.currentPage == nil {
		return fmt.Errorf("session: no current page available")
	}

	// Ensure directory exists
	dirname := session.getDirectory()
	if _, err := os.Stat(dirname); err != nil && os.IsNotExist(err) {
		if err := os.MkdirAll(dirname, os.FileMode(0755)); err != nil {
			return fmt.Errorf("session: failed to create directory %s: %w", dirname, err)
		}
	}

	session.invokeCount++
	filename := session.getHtmlFilename()

	html, err := session.currentPage.Html()
	if err != nil {
		return err
	}

	body := []byte(html)

	// Apply body filter if set (Note: no Response available in unified actions for HTTP scraping)
	if session.BodyFilter != nil {
		filteredBody, err := session.BodyFilter(nil, body)
		if err != nil {
			return fmt.Errorf("session: body filter error: %w", err)
		}
		body = filteredBody
	}

	// Always save HTML
	err = os.WriteFile(filename, body, os.FileMode(0644))
	if err != nil {
		return fmt.Errorf("session: failed to save HTML: %w", err)
	}
	session.Printf("%s SAVE to %v (%v bytes)\n", session.getDebugPrefix(), filename, len(body))

	return nil
}

// extractDataAction extracts data using CSS selectors
func (session *Session) extractDataAction(v interface{}, selector string, opt UnmarshalOption) error {
	session.mu.RLock()
	currentPage := session.currentPage
	session.mu.RUnlock()

	if currentPage == nil {
		return fmt.Errorf("session: no current page available for data extraction")
	}

	selection := currentPage.Find(selector)
	return Unmarshal(v, selection, opt)
}
