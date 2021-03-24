package scraper

import (
	"github.com/PuerkitoBio/goquery"
	"net/url"
	"regexp"
)

// Page holds DOM structure of the page and its URL, Logging information.
type Page struct {
	*goquery.Document
	BaseUrl *url.URL
	Logger  Logger
}

// MetaRefresh returns a URL from "meta http-equiv=refresh" tag if it exists.
// otherwise returns nil.
func (page *Page) MetaRefresh() *url.URL {
	refresh := page.Find("meta[http-equiv=refresh]")
	if refresh.Length() > 0 {
		if content, ok := refresh.Attr("content"); ok {
			urlre := regexp.MustCompile("[uU][rR][lL]=(.*)$")
			submatch := urlre.FindStringSubmatch(content)
			if len(submatch) > 1 {
				url, _ := page.BaseUrl.Parse(submatch[1])
				return url
			}
		}
	}
	return nil
}

// ResolveLink resolve relative URL form the page and returns a full URL.
func (page *Page) ResolveLink(relativeURL string) (string, error) {
	reqUrl, err := page.BaseUrl.Parse(relativeURL)
	if err != nil {
		return "", err
	}
	return reqUrl.String(), nil
}
