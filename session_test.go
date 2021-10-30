package scraper

import (
	"bytes"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/japanese"
	"net/url"
	"reflect"
	"testing"
)

func createHtmlPage(html string) (*Page, error) {
	testUrl, err := url.Parse("http://localhost/")
	if err != nil {
		return nil, err
	}
	logger := &DummyLogger{}
	buf := bytes.NewBufferString(html)
	doc, err := goquery.NewDocumentFromReader(buf)
	if err != nil {
		return nil, err
	}

	page := &Page{doc, testUrl, logger}

	return page, nil
}

func TestGetEncodingFromPageHead(t *testing.T) {
	type args struct {
		html string
	}
	tests := []struct {
		name  string
		args  args
		want  encoding.Encoding
		want1 bool
	}{
		{
			"meta x-sjis",
			args{`<html><head><meta charset="x-sjis"></head></html>"`},
			japanese.ShiftJIS,
			true,
		},
		{
			"http-equiv x-sjis",
			args{`<html><head><meta http-equiv="Content-Type" content="text/html; charset=x-sjis"></head></html>`},
			japanese.ShiftJIS,
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := createHtmlPage(tt.args.html)
			if err != nil {
				t.Error("createHtmlPage", err)
				return
			}
			got, got1 := GetEncodingFromPageHead(page)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetEncodingFromPageHead() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("GetEncodingFromPageHead() got1 = %v, want %v", got1, tt.want1)
			}
		})
	}
}
