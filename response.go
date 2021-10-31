package scraper

import (
	"bytes"
	"encoding/csv"
	"github.com/PuerkitoBio/goquery"
	"github.com/dimchansky/utfbom"
	"golang.org/x/text/encoding"
	"net/http"
	"net/url"
)

// Response holds a raw response and its request information.
type Response struct {
	Request     *http.Request
	ContentType string
	CharSet     string
	Body        []byte
	Encoding    encoding.Encoding
	Logger      Logger
}

// CsvReader returns csv.Reader of the response.
// it assumes the response is a CSV.
func (response *Response) CsvReader() *csv.Reader {
	return csv.NewReader(utfbom.SkipOnly(bytes.NewBuffer(response.Body)))
}

// Page parses raw response to DOM tree and returns a Page object.
func (response *Response) Page() (*Page, error) {
	var doc *goquery.Document
	var err error
	doc, err = goquery.NewDocumentFromReader(bytes.NewBuffer(response.Body))
	if err != nil {
		return nil, err
	}

	if response.Encoding == nil {
		if encode, ok := GetEncodingFromHead(doc); ok {
			response.Logger.Printf("converting from %v...\n", encode)
			b, err := convertEncodingToUtf8(response.Body, encode)
			if err != nil {
				return nil, err
			}
			response.Body = b
			response.Encoding = encode

			// replace doc with converted body
			doc, err = goquery.NewDocumentFromReader(bytes.NewBuffer(response.Body))
			if err != nil {
				return nil, err
			}
		}
	}

	// goquery.NewPageFromResponse だとUrl設定されるが goquery.NewPageFromReader は設定されないので同等にする
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

func GetEncodingFromHead(page *goquery.Document) (encoding.Encoding, bool) {
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
