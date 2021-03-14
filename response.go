package scraper

import (
	"bytes"
	"encoding/csv"
	"github.com/PuerkitoBio/goquery"
	"github.com/dimchansky/utfbom"
	"golang.org/x/text/encoding"
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

type Response struct {
	Request     *http.Request
	ContentType string
	CharSet     string
	Body        []byte
	Encoding    encoding.Encoding
	Logger      Logger
}

func (response *Response) CsvReader() *csv.Reader {
	return csv.NewReader(utfbom.SkipOnly(bytes.NewBuffer(response.Body)))
}

func (response *Response) Page() (*Page, error) {
	var doc *goquery.Document
	var err error
	doc, err = goquery.NewDocumentFromReader(bytes.NewBuffer(response.Body))
	if err != nil {
		return nil, err
	}

	if response.Encoding == nil {
		if content, ok := doc.Find("meta[http-equiv=Content-Type]").Attr("content"); ok {
			m := regexp.MustCompile(`\bcharset=(\w*)`).FindStringSubmatch(strings.ToLower(content))
			if len(m) == 2 {
				if encoding := charsetEncoding(m[1]); encoding != nil {
					response.Logger.Printf("converting from %v...\n", encoding)
					b, err := convertEncodingToUtf8(response.Body, encoding)
					if err != nil {
						return nil, err
					}
					response.Body = b
					response.Encoding = encoding

					// replace doc with converted body
					doc, err = goquery.NewDocumentFromReader(bytes.NewBuffer(response.Body))
					if err != nil {
						return nil, err
					}
				}
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
