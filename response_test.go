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

func createHtmlResponse(html string, htmlEncoding encoding.Encoding, forceEncoding encoding.Encoding, url string, contentType string) (*Response, error) {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	body, err := encode(html, htmlEncoding)
	if err != nil {
		return nil, err
	}

	logger := &DummyLogger{}

	response := &Response{
		Request:     request,
		ContentType: contentType,
		RawBody:     body,
		Encoding:    forceEncoding,
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

func TestGetCharsetFromHead(t *testing.T) {
	type metaInfo struct {
		title  string
		format string
	}
	metas := []metaInfo{
		{"meta charset", `<html><head><meta charset="%v"></head></html>`},
		{"meta http-equiv", `<html><head><meta http-equiv="Content-Type" content="text/html; charset=%v"></head></html>`},
	}

	type test struct {
		name string
		html string
		want string
	}
	tests := make([]test, 0, len(metas))

	for _, meta := range metas {
		tests = append(tests,
			test{
				name: meta.title,
				html: fmt.Sprintf(meta.format, "test-Test?"),
				want: "test-Test?",
			})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			document, err := createHtmlDocument(tt.html)
			if err != nil {
				t.Error("creatHtmlResponse", err)
				return
			}
			got := getCharsetFromHead(document)
			if got != tt.want {
				t.Errorf("want:%v, got:%v", tt.want, got)
			}
		})
	}
}

func TestResponse_Page(t *testing.T) {
	type args struct {
		headHtml      string
		htmlEncoding  encoding.Encoding
		contentType   string
		forceEncoding encoding.Encoding
		url           string
	}
	type result struct {
		Text    string
		BaseUrl string
	}
	type test struct {
		name        string
		args        args
		wantBaseUrl string
	}

	tests := []test{
		{
			name: "plain",
		},
		{
			name: "convert by Encoding",
			args: args{
				htmlEncoding:  japanese.ShiftJIS,
				forceEncoding: japanese.ShiftJIS,
			},
		},
		{
			name: "convert by html head",
			args: args{
				headHtml:     `<meta charset="Shift_JIS">`,
				htmlEncoding: japanese.ShiftJIS,
			},
		},
		{
			name: "convert by content-type",
			args: args{
				htmlEncoding: japanese.ShiftJIS,
				contentType:  "text/html; charset=Shift_JIS",
			},
		},
		{
			name: "Encoding is stronger than meta",
			args: args{
				headHtml:      `<meta charset="EUC-JP">`,
				htmlEncoding:  japanese.ShiftJIS,
				forceEncoding: japanese.ShiftJIS,
			},
		},
		{
			name: "meta is stronger than content-type",
			args: args{
				headHtml:     `<meta charset="Shift_JIS">`,
				htmlEncoding: japanese.ShiftJIS,
				contentType:  "text/html; charset=EUC-JP",
			},
		},
		{
			name: "base URL",
			args: args{
				headHtml: `<base href="http://example.com/">`,
			},
			wantBaseUrl: "http://example.com/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			const localhost = "http://localhost/"

			contentType := tt.args.contentType
			if contentType == "" {
				contentType = "text/html"
			}
			url := tt.args.url
			if url == "" {
				url = localhost
			}
			response, err := createHtmlResponse("<html><head>"+tt.args.headHtml+"</head><body><p>日本語</p></body></html>", tt.args.htmlEncoding, tt.args.forceEncoding, url, contentType)
			if err != nil {
				t.Error("creatHtmlResponse", err)
				return
			}
			page, err := response.Page()
			if err != nil {
				t.Errorf("Page() error = %v", err)
				return
			}
			want := result{
				Text:    "日本語",
				BaseUrl: tt.wantBaseUrl,
			}
			if want.BaseUrl == "" {
				want.BaseUrl = localhost
			}
			got := result{
				page.Find("p").Text(),
				page.BaseUrl.String(),
			}
			if diff := cmp.Diff(want, got); diff != "" {
				t.Errorf("(-want +got)\n%v", diff)
			}
		})
	}
}

func Test_getEncodingFromCharset(t *testing.T) {
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

	type args struct {
		charset string
	}
	type test struct {
		name string
		args args
		want encoding.Encoding
	}
	var tests []test
	for _, charset := range charsets {
		for _, name := range charset.names {
			tests = append(tests, test{
				name,
				args{name},
				charset.encoding,
			})
		}
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getEncodingFromCharset(tt.args.charset); got != tt.want {
				t.Errorf("getEncodingFromCharset() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_charsetFromContentType(t *testing.T) {
	type args struct {
		contentType string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			"plain",
			args{"text/html"},
			"",
		},
		{
			"Shift_JIS",
			args{"text/html; charset=Shift_JIS"},
			"Shift_JIS",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := charsetFromContentType(tt.args.contentType); got != tt.want {
				t.Errorf("charsetFromContentType() = %v, want %v", got, tt.want)
			}
		})
	}
}
