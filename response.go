package scraper

import (
	"bytes"
	"encoding/csv"
	"github.com/PuerkitoBio/goquery"
	"github.com/dimchansky/utfbom"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

// Response holds a raw response and its request information.
type Response struct {
	Request     *http.Request
	ContentType string
	RawBody     []byte
	Encoding    encoding.Encoding
	Logger      Logger
}

// Body returns response body converted from response.Encoding(if not nil).
func (response *Response) Body() ([]byte, error) {
	e := response.Encoding
	if e == nil {
		e = getEncodingFromCharset(charsetFromContentType(response.ContentType))
	}
	if e == nil {
		return response.RawBody, nil
	}
	response.Logger.Printf("converting from %v...\n", e)
	b, _, err := transform.Bytes(e.NewDecoder(), response.RawBody)
	return b, err
}

// CsvReader returns csv.Reader of the response.
// it assumes the response is a CSV.
func (response *Response) CsvReader() *csv.Reader {
	body, err := response.Body()
	if err != nil {
		return nil
	}
	return csv.NewReader(utfbom.SkipOnly(bytes.NewBuffer(body)))
}

type PageOption struct {
	BodyFilter func(resp *Response, body []byte) ([]byte, error)
}

// PageOpt parses raw response to DOM tree and returns a Page object.
func (response *Response) PageOpt(option PageOption) (*Page, error) {
	if response.Encoding == nil {
		doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(response.RawBody))
		if err != nil {
			return nil, err
		}
		e := getEncodingFromCharset(getCharsetFromHead(doc))
		if e != nil {
			response.Encoding = e
		}
	}

	body, err := response.Body()
	if err != nil {
		return nil, err
	}
	if option.BodyFilter != nil {
		body, err = option.BodyFilter(response, body)
		if err != nil {
			return nil, err
		}
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}

	// goquery.NewDocumentFromResponse だとUrl設定されるが goquery.NewDocumentFromReader は設定されないので同等にする
	doc.Url = response.Request.URL
	baseUrl := doc.Url

	base := doc.Find("head base")
	if base.Length() == 1 {
		if href, exists := base.Attr("href"); exists {
			baseUrl, err = url.Parse(href)
			if err != nil {
				return nil, err
			}
		}
	}

	// title
	response.Logger.Printf("* %v\n", doc.Find("title").Text())

	return &Page{doc, baseUrl, response.Logger}, err
}

func (response *Response) Page() (*Page, error) {
	return response.PageOpt(PageOption{})
}

func getCharsetFromHead(document *goquery.Document) string {
	charset := ""

	if metaCharset, exists := document.Find("head meta").Attr("charset"); exists {
		charset = metaCharset
	}

	if metaContentType, exists := document.Find("head meta[http-equiv='Content-Type']").Attr("content"); exists {
		charset = charsetFromContentType(metaContentType)
	}

	return charset
}

func charsetFromContentType(contentType string) string {
	re := regexp.MustCompile(`.*\bcharset=(.*)`)
	match := re.FindStringSubmatch(contentType)
	if len(match) == 2 {
		return match[1]
	}
	return ""
}

// getEncodingFromCharset parses charset string and returns encoding.Encoding.
func getEncodingFromCharset(charset string) encoding.Encoding {
	var e encoding.Encoding
	switch strings.ToLower(charset) {
	case "shift_jis", "windows-31j", "x-sjis", "sjis", "cp932", "shift-jis":
		e = japanese.ShiftJIS
	case "euc-jp":
		e = japanese.EUCJP
	case "iso-2022-jp":
		e = japanese.ISO2022JP
	}
	return e
}
