package scraper

import (
	"bytes"
	"fmt"
	"math"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
)

func createMashallerTestPage(html string) (*Page, error) {
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

type UnmarshalTestData struct {
	NovelURL    string `find:"a.favnovel_hover" attr:"href"`
	Title       string `find:"a.favnovel_hover"`
	BookmarkURL string `find:"span.no a" attr:"href"`
	LatestURL   string `find:"span.favnovel_info a" attr:"href"`
}

func TestUnmarshall(t *testing.T) {
	html := `<div id="favnovel">
	  <div class="favnovel_list">
	    <a href="小説自体URL" class="favnovel_hover"><img />シリーズタイトル抜粋</a>
	    <span class="no">
	      <a href="小説自体URL/しおり回/">n部分</a>
	    </span>
	    <span class="favnovel_info">
	      <a href="小説自体URL/最新回/">最新n部分[完結]</a>
	    </span>
	  </div>
	</div>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	var testData UnmarshalTestData
	err = Unmarshal(&testData, page.Selection, UnmarshalOption{})
	if err != nil {
		t.Error(err)
	}

	name := "UnmarshalTestData"
	shouldBe := UnmarshalTestData{
		NovelURL:    "小説自体URL",
		Title:       "シリーズタイトル抜粋",
		BookmarkURL: "小説自体URL/しおり回/",
		LatestURL:   "小説自体URL/最新回/",
	}
	value := testData
	if value != shouldBe {
		t.Errorf(fmt.Sprintf("%v: %#v != %#v", name, value, shouldBe))
	}

}
func TestUnmarshallInt(t *testing.T) {
	html := `<div><p>42</p><span>123,456</span></div>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	{
		var value int
		shouldBe := 42
		err = Unmarshal(&value, page.Find("p"), UnmarshalOption{})
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(value, shouldBe) {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}

	{
		var value int
		shouldBe := 123456
		err = Unmarshal(&value, page.Find("span"), UnmarshalOption{})
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(value, shouldBe) {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}
}

func TestUnmarshallIntRegEx(t *testing.T) {
	html := `<div>$123US</div>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	{
		var value int
		shouldBe := 123
		err = Unmarshal(&value, page.Selection, UnmarshalOption{Re: `\$([0-9]+)`})
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(value, shouldBe) {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}

	{
		var value int
		shouldBe := 23
		err = Unmarshal(&value, page.Selection, UnmarshalOption{Re: `([32]+)`})
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(value, shouldBe) {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}
}

func TestUnmarshallFloat(t *testing.T) {
	html := `<div>3.14159265</div>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	{
		var value float64
		shouldBe := 3.14159265
		err = Unmarshal(&value, page.Selection, UnmarshalOption{})
		if err != nil {
			t.Error(err)
		}

		if math.Abs(value-shouldBe) > 0.00001 {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}

	// optional way
	{
		var value *float64
		shouldBe := 3.14159265
		err = Unmarshal(&value, page.Selection, UnmarshalOption{})
		if err != nil {
			t.Error(err)
		}

		if value == nil {
			t.Error("value is nil")
		}
		if math.Abs(*value-shouldBe) > 0.00001 {
			t.Errorf(fmt.Sprintf("%#v != %#v", *value, shouldBe))
		}
	}
}

func TestUnmarshallTime(t *testing.T) {
	html := `<div>1986/4/1 12:34</div>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	var value *time.Time
	shouldBe := time.Date(1986, time.April, 1, 12, 34, 0, 0, time.UTC)
	err = Unmarshal(&value, page.Selection, UnmarshalOption{Time: "2006/1/2 03:04"})
	if err != nil {
		t.Error(err)
	}

	if value == nil {
		t.Errorf("value is nil")
	}
	if !shouldBe.Equal(*value) {
		t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
	}
}

func TestUnmarshallArray(t *testing.T) {
	html := `<div><a href="1" /><a href="2" /><a /></div>`
	shouldBe := []string{"1", "2", ""}

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	var value []string
	err = Unmarshal(&value, page.Find("a"), UnmarshalOption{Attr: "href"})
	if err != nil {
		t.Error(err)
	}

	if !reflect.DeepEqual(value, shouldBe) {
		t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
	}
}

func TestUnmarshallOptional(t *testing.T) {
	html := `<div><p>test</p></div>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	var value *string
	var shouldBe *string

	var test = "test"
	shouldBe = &test
	err = Unmarshal(&value, page.Find("p"), UnmarshalOption{})
	if err != nil {
		t.Error(err)
	}
	if value == nil {
		t.Errorf(fmt.Sprintf("%v != %v", value, *shouldBe))
	}
	if *value != *shouldBe {
		t.Errorf(fmt.Sprintf("%v != %v", *value, *shouldBe))
	}

	err = Unmarshal(&value, page.Find("a"), UnmarshalOption{})
	if err != nil {
		t.Error(err)
	}

	shouldBe = nil
	if value != shouldBe {
		t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
	}
}
