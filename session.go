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
	"regexp"
	"strings"
)

const (
	//UserAgent_Chrome39  = "Mozilla/5.0 (Windows NT 6.3; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/39.0.2171.99 Safari/537.36"
	//UserAgent_iOS8      = "Mozilla/5.0 (iPhone; CPU iPhone OS 8_1_3 like Mac OS X) AppleWebKit/600.1.4 (KHTML, like Gecko) Mobile/12B466"
	UserAgent_firefox86 = "Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:86.0) Gecko/20100101 Firefox/86.0"
	UserAgent_default   = UserAgent_firefox86
)

type Session struct {
	Name               string
	client             http.Client
	Encoding           encoding.Encoding
	UserAgent          string
	FilePrefix         string
	invokeCount        int
	NotUseNetwork      bool
	SaveToFile         bool
	ShowRequestHeader  bool
	ShowResponseHeader bool
	ShowFormPosting    bool
	Log                Logger
	jar                *cookiejar.Jar
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
	filename := fmt.Sprintf("%v%v/cookie", session.FilePrefix, session.Name)

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

func (session *Session) SaveCookie() error {
	return session.jar.Save()
}

func charsetEncoding(charset string) encoding.Encoding {
	var encoding encoding.Encoding
	switch strings.ToLower(charset) {
	case "shift_jis", "windows-31j":
		encoding = japanese.ShiftJIS
	case "euc-jp":
		encoding = japanese.EUCJP
	case "iso-2022-jp":
		encoding = japanese.ISO2022JP
	}
	return encoding
}

// convert body(given encoding) to UTF-8.
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

func (session *Session) invoke(req *http.Request) (*Response, error) {
	var body []byte
	var contentType string

	if session.NotUseNetwork || session.SaveToFile {
		if _, err := os.Stat(session.FilePrefix + session.Name); err != nil && os.IsNotExist(err) {
			if err := os.Mkdir(session.FilePrefix+session.Name, os.FileMode(0744)); err != nil {
				return nil, err
			}
		}
	}

	session.invokeCount++
	filename := fmt.Sprintf("%v%v/%v.html", session.FilePrefix, session.Name, session.invokeCount)
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
			return nil, fmt.Errorf("%v request error: %v", req.URL.String(), err)
		}
		defer response.Body.Close()

		req = response.Request // update req.Url after redirects

		if response.StatusCode/100 != 2 {
			return nil, fmt.Errorf("%v response code: %v", req.URL.String(), response.StatusCode)
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

	charSet := regexp.MustCompile(".*charset=(.*)").ReplaceAllString(contentType, "$1")

	encoding := session.Encoding
	if encoding == nil {
		encoding = charsetEncoding(charSet)
	}
	if encoding != nil {
		if session.ShowResponseHeader {
			session.Printf("converting from %v...\n", encoding)
		}
		b, err := convertEncodingToUtf8(body, encoding)
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
		Encoding:    encoding,
		Logger:      session,
	}, nil
}

func (session *Session) Get(getUrl string) (*Response, error) {
	req, err := http.NewRequest("GET", getUrl, nil)
	if err != nil {
		return nil, err
	}
	return session.invoke(req)
}

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
		if url := page.MetaRefresh(); url != nil {
			session.Printf("HTML Meta Refresh to: %v\n", url)
			return session.GetPageMaxRedirect(url.String(), maxRedirect-1)
		}
	}
	return page, nil
}

func (session *Session) ApplyRefresh(page *Page, maxRedirect int) (*Page, error) {
	if maxRedirect > 0 {
		if url := page.MetaRefresh(); url != nil {
			session.Printf("HTML Meta Refresh to: %v\n", url)
			return session.GetPageMaxRedirect(url.String(), maxRedirect-1)
		}
	}
	return page, nil
}

func (session *Session) GetPage(getUrl string) (*Page, error) {
	return session.GetPageMaxRedirect(getUrl, 1)
}

// post a form (easy version)
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

func (session *Session) OpenURL(page *Page, url string) (*Response, error) {
	req, _ := http.NewRequest("GET", url, nil)
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

func (session *Session) Frame(page *Page, frameSelector string) (*Page, error) {
	resp, err := session.FollowLink(page, frameSelector, "src")
	if err != nil {
		return nil, err
	}
	return resp.Page()
}

type FollowAnchorTextOption struct {
	CheckAlt  bool
	TrimSpace bool
	NumLink   int // 0だと個数が1以上なら先頭を返す
	Index     int
}

func MakeFollowAnchorTextOpt2(checkAlt bool, num_link, index int) FollowAnchorTextOption {
	return FollowAnchorTextOption{checkAlt, false, num_link, index}
}

func (session *Session) FollowAnchorTextOpt3(page *Page, text string, opt FollowAnchorTextOption) (*Response, error) {
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

func (session *Session) FollowAnchorTextOpt2(page *Page, text string, checkAlt bool, num_link, index int) (*Response, error) {
	return session.FollowAnchorTextOpt3(page, text, MakeFollowAnchorTextOpt2(checkAlt, num_link, index))
}

func (session *Session) FollowAnchorTextOpt(page *Page, text string, checkAlt bool) (*Response, error) {
	return session.FollowAnchorTextOpt2(page, text, checkAlt, 1, 0)
}

func (session *Session) FollowAnchorTextOptFirst(page *Page, text string, checkAlt bool) (*Response, error) {
	return session.FollowAnchorTextOpt2(page, text, checkAlt, 0, 0)
}

func (session *Session) FollowAnchorText(page *Page, text string) (*Response, error) {
	return session.FollowAnchorTextOpt(page, text, true)
}
