package scraper

import (
	"context"
	"fmt"
	"github.com/chromedp/chromedp"
	"github.com/google/go-cmp/cmp"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
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
<table id="table">
  <tr>
	<td>10</td>
	<td>11</td>
  </tr>	
  <tr>
	<td>20</td>
  </tr>	
  <tr>
	<td>30</td>
	<td>31</td>
  </tr>	
  <tr>
	<td>40</td>
  </tr>	
</table>
</body>
</html>
`,
		)
	}))
	defer ts.Close()

	// create context with CI-aware Chrome options using test helper
	testOptions := NewTestChromeOptions(true)
	allocOptions := []chromedp.ExecAllocatorOption{
		chromedp.UserDataDir("./chromeUserData"), // Basic user data dir
	}
	if testOptions.Headless {
		allocOptions = append(allocOptions,
			chromedp.Headless,
			chromedp.DisableGPU,
		)
	}
	allocOptions = append(allocOptions, testOptions.ExtraAllocOptions...)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOptions...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
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

	type TableOdd struct {
		Odd1 int    `find:"td:nth-of-type(1)"`
		Odd2 string `find:"td:nth-of-type(2)"`
	}
	type TableEven struct {
		Even1 int `find:"td"`
	}
	type TableRecord struct {
		Odd  []TableOdd  `find:"tr:nth-of-type(2n+1)"`
		Even []TableEven `find:"tr:nth-of-type(2n)"`
	}

	type TestRecord struct {
		H1       H1Record    `find:"h1"`
		Num2     int         `find:"div#numbers div:nth-of-type(2)"`
		NumEven  int         `find:"div#numbers div:nth-of-type(even)"`
		NumOdd   []int       `find:"div#numbers div:nth-of-type(odd)"`
		Num2n    int         `find:"div#numbers div:nth-of-type(2n)"`
		Numbers  []int       `find:"div#numbers div" ignore:"3"`
		Date     time.Time   `find:"div#date" time:"2006年1月2日"`
		Optional []optional  `find:"div#optional div"`
		Table    TableRecord `find:"table#table"`
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
		NumEven:  2,
		NumOdd:   []int{1, 3},
		Num2n:    2,
		Numbers:  []int{1, 2, 0},
		Date:     time.Date(2001, time.February, 3, 0, 0, 0, 0, time.UTC),
		Optional: []optional{{Span: &exist}, {Span: nil}},
		Table: TableRecord{
			Odd: []TableOdd{
				{Odd1: 10, Odd2: "11"},
				{Odd1: 30, Odd2: "31"},
			},
			Even: []TableEven{
				{Even1: 20},
				{Even1: 40},
			},
		},
	}
	if !reflect.DeepEqual(record, shouldBe) {
		diff := cmp.Diff(shouldBe, record)
		t.Errorf("ChromeUnmarshal() (-want +got)\n%v", diff)
	}
}

func Test_resolveNthOfType(t *testing.T) {
	type args struct {
		cssSelector string
		n           int
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "odd",
			args: args{
				cssSelector: "div:nth-of-type(odd)",
				n:           1,
			},
			want: "div:nth-of-type(3)",
		},
		{
			name: "even(1)",
			args: args{
				cssSelector: "div:nth-of-type(even)",
				n:           1,
			},
			want: "div:nth-of-type(4)",
		},
		{
			name: "even(0)",
			args: args{
				cssSelector: "div:nth-of-type(even)",
				n:           0,
			},
			want: "div:nth-of-type(2)",
		},
		{
			name: "2n (=even)",
			args: args{
				cssSelector: "div:nth-of-type(2n)",
				n:           1,
			},
			want: "div:nth-of-type(4)",
		},
		{
			name: "2n+1 (=odd)",
			args: args{
				cssSelector: "div:nth-of-type(2n+1)",
				n:           1,
			},
			want: "div:nth-of-type(3)",
		},
		{
			name: "2",
			args: args{
				cssSelector: "div:nth-of-type(2)",
				n:           1,
			},
			want: "div:nth-of-type(2)",
		},
		{
			name: "no nth-of-type",
			args: args{
				cssSelector: "div",
				n:           1,
			},
			want: "div:nth-of-type(2)",
		},
		{
			name: "child tree",
			args: args{
				cssSelector: "div div:nth-of-type(2n+1)",
				n:           1,
			},
			want: "div div:nth-of-type(3)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveNthOfType(tt.args.cssSelector, tt.args.n); got != tt.want {
				t.Errorf("resolveNthOfType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_parseNthOfTypeParam(t *testing.T) {
	type args struct {
		cssSelector string
	}
	tests := []struct {
		name  string
		args  args
		want  string
		want1 int
		want2 int
	}{
		{
			name: "2n+1",
			args: args{
				cssSelector: "test:nth-of-type(2n+1)",
			},
			want:  "test",
			want1: 2,
			want2: 1,
		},
		{
			name: "3n",
			args: args{
				cssSelector: "test:nth-of-type(3n)",
			},
			want:  "test",
			want1: 3,
			want2: 0,
		},
		{
			name: "even",
			args: args{
				cssSelector: "test:nth-of-type(even)",
			},
			want:  "test",
			want1: 2,
			want2: 0,
		},
		{
			name: "1",
			args: args{
				cssSelector: "test:nth-of-type(1)",
			},
			want:  "test",
			want1: 0,
			want2: 1,
		},
		{
			name: "test",
			args: args{
				cssSelector: "test",
			},
			want:  "test",
			want1: 0,
			want2: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, got1, got2 := parseNthOfTypeParam(tt.args.cssSelector)
			if got != tt.want {
				t.Errorf("parseNthOfTypeParam() got = %v, want %v", got, tt.want)
			}
			if got1 != tt.want1 {
				t.Errorf("parseNthOfTypeParam() got1 = %v, want %v", got1, tt.want1)
			}
			if got2 != tt.want2 {
				t.Errorf("parseNthOfTypeParam() got2 = %v, want %v", got2, tt.want2)
			}
		})
	}
}

// CustomUnmarshaller はUnmarshallerインターフェースを実装したテスト用の型
type CustomUnmarshaller struct {
	Value string
}

func (c *CustomUnmarshaller) Unmarshal(s string) error {
	c.Value = "custom:" + s
	return nil
}

// ChromeUnmarshalでUnmarshallerインターフェースが正しく動作することをテストする
func TestChromeUnmarshalWithUnmarshaller(t *testing.T) {
	// create a test server to serve the page
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `
<html>
<body>
<div id="test">hello world</div>
</body>
</html>
`,
		)
	}))
	defer ts.Close()

	// create context with CI-aware Chrome options using test helper
	testOptions := NewTestChromeOptions(true)
	allocOptions := []chromedp.ExecAllocatorOption{
		chromedp.UserDataDir("./chromeUserData"), // Basic user data dir
	}
	if testOptions.Headless {
		allocOptions = append(allocOptions,
			chromedp.Headless,
			chromedp.DisableGPU,
		)
	}
	allocOptions = append(allocOptions, testOptions.ExtraAllocOptions...)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOptions...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := chromedp.Run(ctx, chromedp.Navigate(ts.URL))
	if err != nil {
		t.Fatal(err)
	}

	type TestRecord struct {
		Custom CustomUnmarshaller `find:"div#test"`
	}

	var testRecord TestRecord
	err = ChromeUnmarshal(ctx, &testRecord, "body", UnmarshalOption{})
	if err != nil {
		t.Fatal(err)
	}

	expected := "custom:hello world"
	if testRecord.Custom.Value != expected {
		t.Errorf("CustomUnmarshaller.Value = %v, want %v", testRecord.Custom.Value, expected)
	}
}

// nth-child関連セレクタでエラーが発生することをテストする
func TestChromeUnmarshalNthChildErrors(t *testing.T) {
	// create a test server to serve the page
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `
<html>
<body>
<div>item1</div>
<div>item2</div>
<div>item3</div>
</body>
</html>
`,
		)
	}))
	defer ts.Close()

	// create context with CI-aware Chrome options using test helper
	testOptions := NewTestChromeOptions(true)
	allocOptions := []chromedp.ExecAllocatorOption{
		chromedp.UserDataDir("./chromeUserData"), // Basic user data dir
	}
	if testOptions.Headless {
		allocOptions = append(allocOptions,
			chromedp.Headless,
			chromedp.DisableGPU,
		)
	}
	allocOptions = append(allocOptions, testOptions.ExtraAllocOptions...)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOptions...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := chromedp.Run(ctx, chromedp.Navigate(ts.URL))
	if err != nil {
		t.Fatal(err)
	}

	// nth-childセレクタを使ったスライスでエラーが発生することをテスト
	type TestRecord struct {
		Items []string `find:"div:nth-child(1)"`
	}

	var testRecord TestRecord
	err = ChromeUnmarshal(ctx, &testRecord, "body", UnmarshalOption{})
	if err == nil {
		t.Error("Expected error for nth-child selector in slice field, but got none")
	}
	if !strings.Contains(err.Error(), "nth-child") {
		t.Errorf("Expected error message to contain 'nth-child', but got: %s", err.Error())
	}

	// nth-last-childセレクタを使った場合もテスト
	type TestRecord2 struct {
		Items []string `find:"div:nth-last-child(1)"`
	}

	var testRecord2 TestRecord2
	err = ChromeUnmarshal(ctx, &testRecord2, "body", UnmarshalOption{})
	if err == nil {
		t.Error("Expected error for nth-last-child selector in slice field, but got none")
	}
	if !strings.Contains(err.Error(), "nth-last-child") {
		t.Errorf("Expected error message to contain 'nth-last-child', but got: %s", err.Error())
	}

	// nth-last-of-typeセレクタを使った場合もテスト
	type TestRecord3 struct {
		Items []string `find:"div:nth-last-of-type(1)"`
	}

	var testRecord3 TestRecord3
	err = ChromeUnmarshal(ctx, &testRecord3, "body", UnmarshalOption{})
	if err == nil {
		t.Error("Expected error for nth-last-of-type selector in slice field, but got none")
	}
	if !strings.Contains(err.Error(), "nth-last-of-type") {
		t.Errorf("Expected error message to contain 'nth-last-of-type', but got: %s", err.Error())
	}
}

// first-child, last-childセレクタでnth-of-typeが付与されないことをテストする
func TestChromeUnmarshalFirstLastChildSelectors(t *testing.T) {
	// create a test server to serve the page
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprint(w, `
<html>
<body>
<div>first</div>
<div>middle</div>
<div>last</div>
</body>
</html>
`,
		)
	}))
	defer ts.Close()

	// create context with CI-aware Chrome options using test helper
	testOptions := NewTestChromeOptions(true)
	allocOptions := []chromedp.ExecAllocatorOption{
		chromedp.UserDataDir("./chromeUserData"), // Basic user data dir
	}
	if testOptions.Headless {
		allocOptions = append(allocOptions,
			chromedp.Headless,
			chromedp.DisableGPU,
		)
	}
	allocOptions = append(allocOptions, testOptions.ExtraAllocOptions...)

	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allocOptions...)
	defer allocCancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	ctx, cancel = context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	err := chromedp.Run(ctx, chromedp.Navigate(ts.URL))
	if err != nil {
		t.Fatal(err)
	}

	// first-childとlast-childが正常に動作することを確認
	type TestRecord struct {
		First string `find:"div:first-child"`
		Last  string `find:"div:last-child"`
	}

	var testRecord TestRecord
	err = ChromeUnmarshal(ctx, &testRecord, "body", UnmarshalOption{})
	if err != nil {
		t.Fatal(err)
	}

	if testRecord.First != "first" {
		t.Errorf("Expected 'first', got %s", testRecord.First)
	}
	if testRecord.Last != "last" {
		t.Errorf("Expected 'last', got %s", testRecord.Last)
	}
}
