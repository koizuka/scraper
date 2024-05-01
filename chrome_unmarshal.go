package scraper

import (
	"context"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/chromedp"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func parseNthOfTypeParam(cssSelector string) (string, int, int) {
	// TODO nの係数にマイナスを付けてきたときの対応 (1 ～ bの範囲で % abs(a) == 0 ならOK)
	regex := regexp.MustCompile(`(.*):nth-of-type\((odd|even|(?:(\d+)n)?\+?(\d+)?)\)$`)
	matched := regex.FindStringSubmatch(cssSelector)
	if len(matched) == 0 {
		return cssSelector, 0, 0
	}
	var a int
	var b int
	if matched[3] != "" {
		a, _ = strconv.Atoi(matched[3])
	}
	if matched[4] != "" {
		b, _ = strconv.Atoi(matched[4])
	}
	if matched[2] == "even" {
		a = 2
		b = 0
	} else if matched[2] == "odd" {
		a = 2
		b = 1
	}
	return matched[1], a, b
}

func resolveNthOfType(cssSelector string, n int) string {
	selectors := strings.Split(cssSelector, " ")
	last := selectors[len(selectors)-1]
	prefix, a, b := parseNthOfTypeParam(last)
	x := 1
	if a == 0 && b == 0 {
		x = n + 1
	} else if a == 0 || a == 1 {
		x = b
	} else {
		if b < 1 {
			b = a
		}
		x = n*a + b
	}
	return fmt.Sprintf("%v:nth-of-type(%v)", prefix, x)
}

func fillValue(ctx context.Context, cssSelector string, value reflect.Value, selected []string, opt UnmarshalOption) error {
	// texts
	if value.Kind() == reflect.Slice {
		rv := reflect.MakeSlice(value.Type(), len(selected), len(selected))
		for i := 0; i < len(selected); i++ {
			// TODO nth-child, nth-last-child, nth-last-of-type があったらエラーにする / first-child, last-child があったら nth-of-typeは付けない
			sel := resolveNthOfType(cssSelector, i)
			err := fillValue(ctx, resolveNthOfType(cssSelector, i), rv.Index(i), []string{selected[i]}, opt)
			if err != nil {
				return fmt.Errorf("#%d: %w (sel:%#v)", i, err, sel)
			}
		}
		value.Set(rv)
		return nil
	}

	// pointer
	if value.Kind() == reflect.Ptr {
		if len(selected) == 0 {
			value.Set(reflect.Zero(value.Type()))
			return nil
		}
		newValue := reflect.New(value.Type().Elem())
		value.Set(newValue)
		value = newValue.Elem()
	}

	if len(selected) != 1 {
		return fmt.Errorf("length(%v) != 1", len(selected))
	}

	s := selected[0]

	if opt.Ignore != "" && s == opt.Ignore {
		value.Set(reflect.Zero(value.Type()))
		return nil
	}

	switch value.Interface().(type) {
	case time.Time:
		if opt.Time == "" {
			return fmt.Errorf("time.Time: time tag is required")
		}
		t, err := time.ParseInLocation(opt.Time, s, opt.Loc)
		if err != nil {
			return err
		}
		value.Set(reflect.ValueOf(t))

	default:
		if opt.Time != "" {
			return fmt.Errorf("`time` tag must be empty unless time.Time")
		}
		if !value.CanAddr() {
			return fmt.Errorf("failed CanAddr: %v, %v", value, value.Type())
		}

		if value.Kind() == reflect.Struct {
			return chromeUnmarshalStruct(ctx, value, cssSelector, opt)
		}

		switch value.Interface().(type) {
		case string:
			value.SetString(s)

		case int, int8, int16, int32, int64:
			reComma := regexp.MustCompile(",")
			stripComma := func(s string) string {
				return reComma.ReplaceAllString(s, "")
			}
			var i int64
			_, err := fmt.Sscanf(stripComma(strings.TrimSpace(s)), "%d", &i)
			if err != nil {
				return UnmarshalParseNumberError{err, s}
			}
			value.SetInt(i)

		case uint, uint8, uint16, uint32, uint64:
			reComma := regexp.MustCompile(",")
			stripComma := func(s string) string {
				return reComma.ReplaceAllString(s, "")
			}
			var i uint64
			_, err := fmt.Sscanf(stripComma(strings.TrimSpace(s)), "%d", &i)
			if err != nil {
				return UnmarshalParseNumberError{err, s}
			}
			value.SetUint(i)

		case float32, float64:
			f, err := ExtractNumber(s)
			if err != nil {
				return err
			}
			value.SetFloat(f)

		default:
			return fmt.Errorf("unknown type %v", reflect.TypeOf(value))
		}
	}

	return nil
}

func chromeUnmarshalStruct(ctx context.Context, value reflect.Value, cssSelector string, opt UnmarshalOption) error {
	if opt.Re != "" {
		return fmt.Errorf("`re` tag must be empty for struct")
	}
	if opt.Attr != "" {
		return fmt.Errorf("`attr` tag must be empty for struct")
	}

	var tasks chromedp.Tasks

	const FindTag = "find"
	const AttrTag = "attr"
	const TimeTag = "time"
	const ReTag = "re"
	const HtmlTag = "html"
	const IgnoreTag = "ignore"

	type tempItem struct {
		Text string
		Ok   bool
	}
	type temp struct {
		Nodes []cdp.NodeID
		Texts []tempItem
	}
	vt := value.Type()

	tempValues := make([]temp, vt.NumField())

	// collect NodeIDs
	for i := 0; i < vt.NumField(); i++ {
		fieldType := vt.Field(i)

		selector := fieldType.Tag.Get(FindTag)

		query := cssSelector
		if selector != "" {
			query = fmt.Sprintf("%v %v", query, selector)
		}
		tasks = append(tasks, chromedp.NodeIDs(query, &tempValues[i].Nodes, chromedp.AtLeast(0)))
	}
	if err := chromedp.Run(ctx, tasks); err != nil {
		return err
	}

	// get texts
	tasks = []chromedp.Action{}
	for i := 0; i < vt.NumField(); i++ {
		fieldType := vt.Field(i)

		tempValues[i].Texts = make([]tempItem, len(tempValues[i].Nodes))
		for j, nodeId := range tempValues[i].Nodes {
			nodeIds := []cdp.NodeID{nodeId}
			var action chromedp.Action
			result := &tempValues[i].Texts[j]
			_, isHtml := fieldType.Tag.Lookup(HtmlTag)
			if isHtml {
				result.Ok = true
				action = chromedp.InnerHTML(nodeIds, &result.Text, chromedp.ByNodeID)
			} else {
				attr := fieldType.Tag.Get(AttrTag)
				if attr != "" {
					action = chromedp.AttributeValue(nodeIds, attr, &result.Text, &result.Ok, chromedp.ByNodeID)
				} else {
					result.Ok = true
					action = chromedp.Text(nodeIds, &result.Text, chromedp.ByNodeID)
				}
			}
			tasks = append(tasks, action)
		}
	}
	if err := chromedp.Run(ctx, tasks); err != nil {
		return err
	}

	// fill results
	for i := 0; i < vt.NumField(); i++ {
		fieldType := vt.Field(i)
		fieldValue := value.Field(i)

		optRe := fieldType.Tag.Get(ReTag)
		var selected []string
		for _, text := range tempValues[i].Texts {
			if !text.Ok {
				continue
			}

			s := text.Text

			// 正規表現パターンがあったら適用する
			if optRe != "" {
				re, err := regexp.Compile(optRe)
				if err != nil {
					return fmt.Errorf("re:%#v: %v", optRe, err)
				}
				submatch := re.FindStringSubmatch(s)
				n := len(submatch) - 1
				if n == -1 {
					continue
				} else if n != 1 {
					return fmt.Errorf("re:%#v: matched count of the regular expression is %d, should be 0 or 1, for text %#v", optRe, n, s)
				} else {
					s = submatch[1]
				}
			}
			selected = append(selected, s)
		}

		if fieldType.PkgPath != "" {
			return UnmarshalFieldError{
				fieldType.Name,
				UnmarshalUnexportedFieldError{},
			}
		}

		opt.Time = fieldType.Tag.Get(TimeTag)
		opt.Attr = fieldType.Tag.Get(AttrTag)
		opt.Ignore = fieldType.Tag.Get(IgnoreTag)
		optFind := fieldType.Tag.Get(FindTag)

		err := fillValue(ctx, fmt.Sprintf("%v %v", cssSelector, optFind), fieldValue, selected, opt)
		if err != nil {
			return UnmarshalFieldError{
				fieldType.Name,
				err,
			}
		}
	}

	return nil
}

func ChromeUnmarshal(ctx context.Context, v interface{}, cssSelector string, opt UnmarshalOption) error {
	if opt.Loc == nil {
		opt.Loc = time.UTC
	}

	ht := reflect.TypeOf(v)
	if ht.Kind() != reflect.Ptr {
		return UnmarshalMustBePointerError{}
	}

	// struct専用
	value := reflect.ValueOf(v).Elem()
	if !value.CanAddr() {
		return errors.New("must be address")
	}
	if value.Kind() != reflect.Struct {
		return errors.New("must be struct")
	}
	if opt.Time != "" {
		return fmt.Errorf("`time` tag must be empty unless time.Time")
	}
	return chromeUnmarshalStruct(ctx, value, cssSelector, opt)
}
