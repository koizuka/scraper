package scraper

import (
	"github.com/PuerkitoBio/goquery"
	"net/url"
	"regexp"
)

type Page struct {
	*goquery.Document
	BaseUrl *url.URL
	Logger  Logger
}

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

func (page *Page) ResolveLink(relativeURL string) (string, error) {
	reqUrl, err := page.BaseUrl.Parse(relativeURL)
	if err != nil {
		return "", err
	}
	return reqUrl.String(), nil
}
