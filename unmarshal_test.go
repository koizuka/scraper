package scraper

import (
	"bytes"
	"errors"
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
	html := `<div><p>42</p><span id="int">123,456</span><span id="uint">654321</span></div>`

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
		err = Unmarshal(&value, page.Find("span#int"), UnmarshalOption{})
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(value, shouldBe) {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}

	{
		var value uint
		var shouldBe uint = 654321
		err = Unmarshal(&value, page.Find("span#uint"), UnmarshalOption{})
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

	{
		var value int
		shouldBe := `expected integer: "US"`
		err = Unmarshal(&value, page.Selection, UnmarshalOption{Re: `(US)`})
		if err == nil {
			t.Error(fmt.Errorf("must be an error"))
		} else {
			errorString := err.Error()
			if errorString != shouldBe {
				t.Errorf(fmt.Sprintf("%#v != %#v", errorString, shouldBe))
			}
		}
	}

	{
		var value *string
		var shouldBe *string = nil
		err = Unmarshal(&value, page.Selection, UnmarshalOption{Re: `(nothing)`})
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(value, shouldBe) {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}
}

func TestUnmarshallFloat(t *testing.T) {
	html := `<div>3.14159265</div><span>test</span>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	{
		var value float64
		shouldBe := 3.14159265
		err = Unmarshal(&value, page.Selection.Find("div"), UnmarshalOption{})
		if err != nil {
			t.Error(err)
		}

		if math.Abs(value-shouldBe) > 0.00001 {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}

	// error
	{
		var value float64
		shouldBe := `strconv.ParseFloat: parsing "test": invalid syntax`
		err = Unmarshal(&value, page.Selection.Find("span"), UnmarshalOption{})
		if err == nil {
			t.Error(fmt.Errorf("shoulb be an error"))
		} else {
			errorString := err.Error()
			if errorString != shouldBe {
				t.Errorf(fmt.Sprintf("%#v != %#v", errorString, shouldBe))
			}
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
	type table struct {
		HTML      string
		Format    string
		Location  *time.Location
		Expect    time.Time
		ExpectErr string
	}
	loc, _ := time.LoadLocation("Asia/Tokyo")

	testData := []table{
		{
			HTML:   `<div>1986/4/1 12:34</div>`,
			Format: "2006/1/2 03:04",
			Expect: time.Date(1986, time.April, 1, 12, 34, 0, 0, time.UTC),
		},
		{
			HTML:     `1999/04/01 12:34`,
			Format:   "2006/01/02 03:04",
			Location: loc,
			Expect:   time.Date(1999, time.April, 1, 12, 34, 0, 0, loc),
		},
		{
			HTML:      "abc",
			Format:    "2006/1/2 03:04",
			ExpectErr: `parsing time "abc" as "2006/1/2 03:04": cannot parse "abc" as "2006"`,
		},
	}

	for i, item := range testData {
		html := item.HTML
		format := item.Format
		location := item.Location
		shouldBe := item.Expect

		page, err := createMashallerTestPage(html)
		if err != nil {
			t.Error(fmt.Sprintf("%v: prepare html(%#v): %v", i, html, err))
			continue
		}

		var value *time.Time
		err = Unmarshal(&value, page.Selection, UnmarshalOption{Time: format, Loc: location})
		if item.ExpectErr == "" {
			if err != nil {
				t.Error(fmt.Sprintf("%v: Unmarshal(%#v, %#v): %v", i, html, format, err))
				continue
			}

			if value == nil {
				t.Errorf(fmt.Sprintf("%v: Unmarshal(%#v, %#v): value is nil", i, html, format))
				continue
			}

			if !shouldBe.Equal(*value) {
				t.Errorf(fmt.Sprintf("%v: Unmarshal(%#v, %#v): %#v != %#v", i, html, format, value, shouldBe))
			}
		} else {
			s := err.Error()
			if item.ExpectErr != s {
				t.Errorf(fmt.Sprintf("%v: Unmarshal(%#v, %#v): error %#v != %#v", i, html, format, s, item.ExpectErr))
			}
		}
	}
}

func TestUnmarshallArray(t *testing.T) {
	html := `<div><a href="1" /><a href="2" /><a /></div>`
	shouldBe := []string{"1", "2"}

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

type EmptyFind struct {
	Href string `attr:"href"`
	Text string
}

func TestUnmarshallEmptyFind(t *testing.T) {
	html := `<a href="URL">text</a>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	sel := page.Find("a")

	{
		var value string
		err = Unmarshal(&value, sel, UnmarshalOption{Attr: "href"})
		if err != nil {
			t.Error(err)
		}
		shouldBe := "URL"
		if value != shouldBe {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}

	{
		var value EmptyFind
		err = Unmarshal(&value, sel, UnmarshalOption{})
		if err != nil {
			t.Error(err)
		}

		shouldBe := EmptyFind{
			Href: "URL",
			Text: "text",
		}
		if value != shouldBe {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}
}

func TestUnmarshalFieldError_Error(t *testing.T) {
	target := UnmarshalFieldError{
		"a",
		UnmarshalFieldError{
			"b",
			errors.New("test"),
		},
	}

	shouldBe := "a.b: test"
	value := target.Error()
	if value != shouldBe {
		t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
	}
}

type UnmarshalTestData2Item struct {
	Text string
}
type UnmarshalTestData2 struct {
	P []UnmarshalTestData2Item `find:"p"`
}

func TestUnmarshallStructArrayInStruct(t *testing.T) {
	html := `<div> <p>1</p> <p>2</p> <p>3</p> <p>4</p> </div>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	{
		var value UnmarshalTestData2
		shouldBe := []UnmarshalTestData2Item{{"1"}, {"2"}, {"3"}, {"4"}}
		err = Unmarshal(&value, page.Find("div"), UnmarshalOption{})
		if err != nil {
			t.Error(err)
		}

		if !reflect.DeepEqual(value.P, shouldBe) {
			t.Errorf(fmt.Sprintf("%#v != %#v", value, shouldBe))
		}
	}
}

type UnmarshalTestUnexportedField struct {
	test string
}

func TestUnmarshallUnexportedField(t *testing.T) {
	html := `<div></div>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	{
		var value UnmarshalTestUnexportedField
		value.test = ""
		shouldBe := UnmarshalFieldError{"test", UnmarshalUnexportedFieldError{}}
		err = Unmarshal(&value, page.Find("div"), UnmarshalOption{})
		if err != shouldBe {
			t.Errorf(fmt.Sprintf("'%v' != '%v'", err, shouldBe))
		}
	}
}

func TestUnmarshallHtml(t *testing.T) {
	html := `<div><a href="https://example.com">link</a><p>p</p></div>`
	shouldBe := `<a href="https://example.com">link</a><p>p</p>`

	page, err := createMashallerTestPage(html)
	if err != nil {
		t.Error(err)
	}

	name := "UnmarshalHtml value"
	var value string
	err = Unmarshal(&value, page.Find("div"), UnmarshalOption{Html: true})
	if err != nil {
		t.Error(err)
	}
	if value != shouldBe {
		t.Errorf(fmt.Sprintf("%v: %#v != %#v", name, value, shouldBe))
	}

	name = "UnmarshalHtml tagValue"
	type TagValue struct {
		Html string `find:"div" html:""`
	}
	var tagValue TagValue
	err = Unmarshal(&tagValue, page.Selection, UnmarshalOption{})
	if err != nil {
		t.Error(err)
	}
	if tagValue.Html != shouldBe {
		t.Errorf(fmt.Sprintf("%v: %#v != %#v", name, tagValue.Html, shouldBe))
	}
}
