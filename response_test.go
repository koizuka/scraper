package scraper

import (
	"bytes"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/google/go-cmp/cmp"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
	"io/ioutil"
	"net/http"
	"strings"
	"testing"
)

func createHtmlResponse(html string, encoding encoding.Encoding, baseURL string) (*Response, error) {
	request, err := http.NewRequest("GET", baseURL, nil)
	if err != nil {
		return nil, err
	}
	body, err := encode(html, encoding)
	if err != nil {
		return nil, err
	}

	logger := &DummyLogger{}

	response := &Response{
		Request:     request,
		ContentType: "text/html",
		CharSet:     "",
		Body:        body,
		Logger:      logger,
	}
	return response, nil
}

func createHtmlDocument(html string) (*goquery.Document, error) {
	buf := bytes.NewBufferString(html)
	return goquery.NewDocumentFromReader(buf)
}

func encode(s string, encoding encoding.Encoding) ([]byte, error) {
	if encoding == nil {
		return []byte(s), nil
	}
	return ioutil.ReadAll(transform.NewReader(strings.NewReader(s), encoding.NewEncoder()))
}

func encodingName(e encoding.Encoding) string {
	switch e {
	case japanese.ShiftJIS:
		return "japanese.ShiftJIS"
	case japanese.EUCJP:
		return "japanese.EUCJP"
	case japanese.ISO2022JP:
		return "japanese.ISO2022JP"
	case nil:
		return "nil"
	default:
		return "unknown"
	}
}

func TestGetEncodingFromHead(t *testing.T) {
	type charsetInfo struct {
		encoding encoding.Encoding
		names    []string
	}
	charsets := []charsetInfo{
		{nil, []string{"UTF-8", "unknown"}},
		{japanese.ShiftJIS, []string{
			"Shift_JIS",
			"windows-31j",
			"cp932",
			"shift-jis",
			"sjis",
			"x-sjis",
		}},
		{japanese.EUCJP, []string{"EUC-JP"}},
		{japanese.ISO2022JP, []string{"ISO-2022-JP"}},
	}

	type metaInfo struct {
		title  string
		format string
	}
	metas := []metaInfo{
		{"meta charset", `<html><head><meta charset="%v"></head></html>`},
		{"meta http-equiv", `<html><head><meta http-equiv="Content-Type" content="%v"></head></html>`},
	}

	type test struct {
		name string
		html string
		want encoding.Encoding
	}
	var tests []test

	for _, meta := range metas {
		for _, charset := range charsets {
			for _, name := range charset.names {
				tests = append(tests,
					test{
						name: fmt.Sprintf("%v %v", name, meta.title),
						html: fmt.Sprintf(meta.format, name),
						want: charset.encoding,
					})
			}
		}
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := createHtmlDocument(tt.html)
			if err != nil {
				t.Error("creatHtmlResponse", err)
				return
			}
			got, _ := GetEncodingFromHead(document)
			if got != tt.want {
				t.Errorf("want:%v, got:%v", encodingName(tt.want), encodingName(got))
			}
		})
	}
}

func TestResponse_Page(t *testing.T) {
	type result struct {
		Text    string
		BaseUrl string
	}
	type test struct {
		name      string
		html      string
		url       string
		encoding  encoding.Encoding
		wantQuery string
		want      result
	}
	tests := []test{
		{
			name:      "plain",
			html:      `<body><p>日本語</p></body>`,
			url:       "http://localhost/",
			encoding:  nil,
			wantQuery: "p",
			want:      result{"日本語", "http://localhost/"},
		},
		{
			name:      "converted",
			html:      `<head><meta charset="Shift_JIS"></head><body><p>日本語</p></body>`,
			url:       "http://localhost/",
			encoding:  japanese.ShiftJIS,
			wantQuery: "p",
			want:      result{"日本語", "http://localhost/"},
		},
		{
			name:      "base URL",
			html:      `<head><base href="http://example.com/"></base><body><p></p></body>`,
			url:       "http://localhost/",
			encoding:  nil,
			wantQuery: "p",
			want:      result{"", "http://example.com/"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			response, err := createHtmlResponse(tt.html, tt.encoding, tt.url)
			if err != nil {
				t.Error("creatHtmlResponse", err)
				return
			}
			page, err := response.Page()
			if err != nil {
				t.Errorf("Page() error = %v", err)
				return
			}
			got := result{
				page.Find(tt.wantQuery).Text(),
				page.BaseUrl.String(),
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("(-want +got)\n%v", diff)
			}
		})
	}
}
