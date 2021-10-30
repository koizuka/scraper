package scraper

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	cookiejar "github.com/orirawlings/persistent-cookiejar"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"
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

// charsetEncoding parses chatset string and returns encoding.Encoding.
func charsetEncoding(charset string) encoding.Encoding {
	var encode encoding.Encoding
	switch strings.ToLower(charset) {
	case "shift_jis", "windows-31j", "x-sjis":
		encode = japanese.ShiftJIS
	case "euc-jp":
		encode = japanese.EUCJP
	case "iso-2022-jp":
		encode = japanese.ISO2022JP
	}
	return encode
}

// convetEncodingToUtf8 converts body(given encoding) to UTF-8.
func convertEncodingToUtf8(body []byte, encoding encoding.Encoding) ([]byte, error) {
	if encoding == nil {
		return body, nil
	}
	b, _, err := transform.Bytes(encoding.NewDecoder(), body)
	if err != nil {
		return nil, err
	}
	return b, nil
}

func (session *Session) getDirectory() string {
	return fmt.Sprintf("%v%v", session.FilePrefix, session.Name)
}

func (session *Session) getHtmlFilename() string {
	return path.Join(session.getDirectory(), fmt.Sprintf("%v.html", session.invokeCount))
}

func charsetFromContentType(contentType string) string {
	charSet := regexp.MustCompile(".*charset=(.*)").ReplaceAllString(contentType, "$1")
	return charSet
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
	contentTypeFilename := filename + ".ContentType"

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
		defer response.Body.Close()

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

		body, err = ioutil.ReadAll(response.Body)
		if err != nil {
			return nil, err
		}

		if session.SaveToFile {
			// save to file
			session.Printf("**** SAVE to %v (%v bytes)\n", filename, len(body))
			err = ioutil.WriteFile(filename, body, os.FileMode(0644))
			if err != nil {
				return nil, err
			}

			err = ioutil.WriteFile(contentTypeFilename, []byte(contentType), os.FileMode(0644))
			if err != nil {
				return nil, err
			}
		}
	} else {
		// load from file
		session.Printf("**** LOAD from %v\n", filename)
		var err error
		body, err = ioutil.ReadFile(filename)
		if err != nil {
			return nil, RetryAndRecordError{filename}
		}

		ct, err := ioutil.ReadFile(contentTypeFilename)
		if err != nil {
			return nil, RetryAndRecordError{filename}
		}
		contentType = string(ct)
	}

	if session.ShowResponseHeader {
		session.Printf("Content-type: %v\n", contentType)
	}

	charSet := charsetFromContentType(contentType)

	encode := session.Encoding
	if encode == nil {
		encode = charsetEncoding(charSet)
	}
	if encode != nil {
		if session.ShowResponseHeader {
			session.Printf("converting from %v...\n", encode)
		}
		b, err := convertEncodingToUtf8(body, encode)
		if err != nil {
			return nil, err
		}
		body = b
	}

	return &Response{
		Request:     req,
		ContentType: contentType,
		CharSet:     charSet,
		Body:        body,
		Encoding:    encode,
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

func GetEncodingFromPageHead(page *Page) (encoding.Encoding, bool) {
	newCharset := ""

	metaCharset, exists := page.Find("head meta").Attr("charset")
	if exists {
		newCharset = metaCharset
	}

	metaContentType, exists := page.Find("head meta[http-equiv='Content-Type']").Attr("content")
	if exists {
		newCharset = charsetFromContentType(metaContentType)
	}

	if newCharset != "" {
		return charsetEncoding(newCharset), true
	} else {
		return nil, false
	}
}

// ApplyMetaCharset converts if head/meta.charset exists in the page, and it states different from Content-Type/charset of the response header

func (session *Session) ApplyMetaCharset(resp *Response, page *Page) (*Page, error) {
	metaEncoding, exists := GetEncodingFromPageHead(page)
	if exists && metaEncoding != resp.Encoding {
		// convert
		b, err := convertEncodingToUtf8(resp.Body, metaEncoding)
		if err != nil {
			return nil, err
		}
		resp.Body = b
		resp.Encoding = metaEncoding
		page, err = resp.Page()
		if err != nil {
			return nil, err
		}
	}

	return page, nil
}

// GetPageMaxRedirect gets the URL and follows HTTP meta refresh if response page contained that.
func (session *Session) GetPageMaxRedirect(getUrl string, maxRedirect int) (*Page, error) {
	resp, err := session.Get(getUrl)
	if err != nil {
		return nil, err
	}
	page, err := resp.Page()
	if err != nil {
		return nil, err
	}
	if maxRedirect > 0 {
		if newUrl := page.MetaRefresh(); newUrl != nil {
			session.Printf("HTML Meta Refresh to: %v\n", newUrl)
			return session.GetPageMaxRedirect(newUrl.String(), maxRedirect-1)
		}
	}
	return session.ApplyMetaCharset(resp, page)
}

// ApplyRefresh mimics HTML Meta Refresh.
func (session *Session) ApplyRefresh(page *Page, maxRedirect int) (*Page, error) {
	if maxRedirect > 0 {
		if newUrl := page.MetaRefresh(); newUrl != nil {
			session.Printf("HTML Meta Refresh to: %v\n", newUrl)
			return session.GetPageMaxRedirect(newUrl.String(), maxRedirect-1)
		}
	}
	return page, nil
}

// GetPage gets the URL and returns a Page.
func (session *Session) GetPage(getUrl string) (*Page, error) {
	return session.GetPageMaxRedirect(getUrl, 1)
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
	return resp.Page()
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
