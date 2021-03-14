package scraper

import (
	"bytes"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"net/url"
	"testing"
)

type DummyLogger struct{}

func (logger DummyLogger) Printf(format string, a ...interface{}) {}

type FormPageTestData struct {
	Title                      string
	Html                       string
	FormSelector               string
	ValueShouldBe              map[string]string
	LabelShouldBe              map[string]string
	NumAvailableValuesShouldBe map[string]int
}

// test Page::Form()
func TestFormPage(t *testing.T) {
	testData := []FormPageTestData{
		// TODO add more
		{
			"input without label",
			"<form><input name='name1' value='value1'></input></form>",
			"form",
			map[string]string{"name1": "value1"},
			map[string]string{"name1": ""},
			map[string]int{"name1": 0},
		},
		{
			"input with label",
			"<form><input name='name1' value='value1' id='id1'></input><label for='id1'>label1</label></form>",
			"form",
			map[string]string{"name1": "value1"},
			map[string]string{"name1": "label1"},
			map[string]int{"name1": 0},
		},
		{
			"input radio single without checked -> check first one",
			"<form><input name='name1' type='radio' value='value1'></input></form>",
			"form",
			map[string]string{"name1": "value1"},
			map[string]string{"name1": ""},
			map[string]int{"name1": 1},
		},
		{
			"input radio multiple without checked -> check first one",
			"<form><input name='name1' type='radio' value='value1' /><input name='name1' type='radio' value='value2' /></form>",
			"form",
			map[string]string{"name1": "value1"},
			map[string]string{"name1": ""},
			map[string]int{"name1": 2},
		},
		{
			"input radio multiple with checked -> check specified one",
			"<form><input name='name1' type='radio' value='value1' /><input name='name1' type='radio' value='value2' checked /></form>",
			"form",
			map[string]string{"name1": "value2"},
			map[string]string{"name1": ""},
			map[string]int{"name1": 2},
		},
		{
			"input radio single with label",
			"<form><input name='name1' type='radio' value='value1' id='id1'></input><label for='id1'>label1</label></form>",
			"form",
			map[string]string{"name1": "value1"},
			map[string]string{"name1": "label1"},
			map[string]int{"name1": 1},
		},
		{
			"select/option single without selected -> select first one",
			"<form><select name='name1'><option value='value1'>label1</option></select></form>",
			"form",
			map[string]string{"name1": "value1"},
			map[string]string{"name1": "label1"},
			map[string]int{"name1": 1},
		},
		{
			"select/option multiple with select -> select specified one",
			"<form><select name='name1'><option value='value1'>label1</option><option value='value2' selected>label2</option></select></form>",
			"form",
			map[string]string{"name1": "value2"},
			map[string]string{"name1": "label2"},
			map[string]int{"name1": 2},
		},
		{
			"select/option multiple without select -> select first one",
			"<form><select name='name1'><option value='value1'>label1</option><option value='value2'>label2</option></select></form>",
			"form",
			map[string]string{"name1": "value1"},
			map[string]string{"name1": "label1"},
			map[string]int{"name1": 2},
		},
	}

	testUrl, err := url.Parse("http://localhost/")
	if err != nil {
		t.Error(err)
	}
	logger := &DummyLogger{}

	for _, test := range testData {
		i := test.Title
		buf := bytes.NewBufferString(test.Html)
		doc, err := goquery.NewDocumentFromReader(buf)
		if err != nil {
			t.Error(err)
		}

		page := &Page{doc, testUrl, logger}

		form, err := page.Form(test.FormSelector)
		if err != nil {
			t.Error(err)
		}

		for name, shouldBe := range test.ValueShouldBe {
			elem := form.Elements[name]
			if elem == nil {
				t.Errorf(fmt.Sprintf("%v: form.Elements[%v] == nil", i, name))
			} else if elem.Value == nil {
				t.Errorf(fmt.Sprintf("%v: avaiableValue of %v == nil", i, name))
			} else if elem.Value.Value != shouldBe {
				t.Errorf(fmt.Sprintf("%v: value of %v: %v != %v", i, name, shouldBe, elem.Value.Value))
			}
		}
		for name, shouldBe := range test.LabelShouldBe {
			elem := form.Elements[name]
			if elem == nil {
				t.Errorf(fmt.Sprintf("%v: form.Elements[%v] == nil", i, name))
			} else if elem.Value == nil {
				t.Errorf(fmt.Sprintf("%v: avaiableValue of %v == nil", i, name))
			} else if elem.Value.Label != shouldBe {
				t.Errorf(fmt.Sprintf("%v: label of %v: %v != %v", i, name, shouldBe, elem.Value.Value))
			}
		}
		for name, shouldBe := range test.NumAvailableValuesShouldBe {
			elem := form.Elements[name]
			if len(elem.AvailableValues) != shouldBe {
				t.Errorf("%v: number ov availableValues != %v", i, shouldBe)
			}
		}
	}
}
