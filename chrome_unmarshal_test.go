package scraper

import (
	"context"
	"fmt"
	"github.com/chromedp/chromedp"
	"github.com/google/go-cmp/cmp"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
	"time"
)

func TestChromeUnmarshal(t *testing.T) {
	// create a test server to serve the page
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `
<html lang="en">
<head>
    <meta charset="UTF-8">
    <title>Title</title>
</head>
<body>
<h1 id="title" class="link">
    <a href="https://test.com/helloworld">
        content of h1 1
    </a>
    <span>hello</span> world
</h1>
<div id="numbers">
  <div>1</div>
  <div>2</div>
  <div>3</div>
</div>
<div id="date">2001年2月3日</div>
<div id="optional">
  <div>
    <span>exist</span>
  </div>
  <div>
  </div>
</div>
</body>
</html>
`,
		)
	}))
	defer ts.Close()

	// create context
	ctx, cancel := chromedp.NewContext(context.Background())
	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := chromedp.Run(ctx, chromedp.Navigate(ts.URL))
	if err != nil {
		t.Fatal(err)
	}

	type H1Record struct {
		Id    string `attr:"id"`
		Class string `attr:"class"`
		Hello string `find:"span"`
		El    string `find:"span" re:"(el)"`
	}

	type optional struct {
		Span *string `find:"span"`
	}

	type TestRecord struct {
		H1       H1Record   `find:"h1"`
		Num2     int        `find:"div#numbers div:nth-of-type(2)"`
		Numbers  []int      `find:"div#numbers div" ignore:"3"`
		Date     time.Time  `find:"div#date" time:"2006年1月2日"`
		Optional []optional `find:"div#optional div"`
	}

	var record TestRecord

	err = ChromeUnmarshal(ctx, &record, "body", UnmarshalOption{})
	if err != nil {
		t.Fatal(err)
	}
	err = chromedp.Cancel(ctx)
	if err != nil {
		t.Fatal(err)
	}

	exist := "exist"
	shouldBe := TestRecord{
		H1: H1Record{
			Id:    "title",
			Class: "link",
			Hello: "hello",
			El:    "el",
		},
		Num2:     2,
		Numbers:  []int{1, 2, 0},
		Date:     time.Date(2001, time.February, 3, 0, 0, 0, 0, time.UTC),
		Optional: []optional{{Span: &exist}, {Span: nil}},
	}
	if !reflect.DeepEqual(record, shouldBe) {
		diff := cmp.Diff(shouldBe, record)
		t.Errorf("ChromeUnmarshal() (-want +got)\n%v", diff)
	}
}
