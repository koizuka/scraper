package scraper

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

type Unmarshaller interface {
	Unmarshal(s string) error
}

type UnmarshalMustBePointerError struct{}

func (err UnmarshalMustBePointerError) Error() string {
	return "must be a pointer to the value"
}

type UnmarshalUnexportedFieldError struct{}

func (err UnmarshalUnexportedFieldError) Error() string {
	return "field must be exported"
}

type UnmarshalFieldError struct {
	Field string
	Err   error
}

func (err UnmarshalFieldError) Error() string {
	e := err.Err
	fields := []string{err.Field}
	next, ok := e.(UnmarshalFieldError)
	for ok {
		fields = append(fields, next.Field)
		e = next.Err
		next, ok = e.(UnmarshalFieldError)
	}
	return fmt.Sprintf("%v: %v", strings.Join(fields, "."), e)
}

func stripchars(str, chr string) string {
	return strings.Map(func(r rune) rune {
		if !strings.ContainsRune(chr, r) {
			return r
		}
		return -1
	}, str)
}

func ExtractNumber(in string) (float64, error) {
	re := regexp.MustCompile(" *([0-9,]+([.][0-9]*)?).*")
	s := stripchars(re.ReplaceAllString(in, "$1"), ",\u00a0\u3000")
	return strconv.ParseFloat(s, 64)
}

type UnmarshalOption struct {
	Attr string         // if nonempty, extracts attribute of element. otherwise, uses Text()
	Re   string         // Regular Expression. must contain one capture.
	Time string         // for time.Time only. parse with this format.
	Loc  *time.Location // time zone for parsing time.Time.
}

func unmarshalValue(value reflect.Value, sel *goquery.Selection, opt UnmarshalOption) error {
	if !value.CanSet() {
		return errors.New("value must CanSet")
	}

	type pair struct {
		Sel  *goquery.Selection
		Text string
	}
	selected := make([]pair, 0, sel.Length())
	for i := 0; i < sel.Length(); i++ {
		j := sel.Eq(i)

		// extract text from Attr(attr) or Text()
		var s string
		if opt.Attr != "" {
			if w, ok := j.Attr(opt.Attr); ok {
				s = w
			} else {
				continue
			}
		} else {
			s = j.Text()
		}

		// 正規表現パターンがあったら適用する
		if opt.Re != "" {
			re, err := regexp.Compile(opt.Re)
			if err != nil {
				return fmt.Errorf("re:%#v: %v", opt.Re, err)
			}
			submatch := re.FindStringSubmatch(s)
			n := len(submatch) - 1
			if n == -1 {
				continue
			} else if n != 1 {
				return fmt.Errorf("re:%#v: matched count of the regular expression is %d, should be 0 or 1, for text %#v", opt.Re, n, s)
			} else {
				s = submatch[1]
			}
		}

		selected = append(selected, pair{j, s})
	}

	if value.Kind() == reflect.Slice {
		rv := reflect.MakeSlice(value.Type(), len(selected), len(selected))
		for i := 0; i < len(selected); i++ {
			err := unmarshalValueOne(rv.Index(i), selected[i].Sel, selected[i].Text, opt)
			if err != nil {
				return fmt.Errorf("#%d: %v", i, err)
			}
		}
		value.Set(rv)
		return nil
	}

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

	return unmarshalValueOne(value, selected[0].Sel, selected[0].Text, opt)
}

func unmarshalValueOne(value reflect.Value, sel *goquery.Selection, s string, opt UnmarshalOption) error {
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

		// その型が Unmarshaller を実装しているならそれを呼ぶ
		if inf, ok := value.Addr().Interface().(Unmarshaller); ok {
			return inf.Unmarshal(s)
		}

		if value.Kind() == reflect.Struct {
			if opt.Re != "" {
				return fmt.Errorf("`re` tag must be empty for struct")
			}
			if opt.Attr != "" {
				return fmt.Errorf("`attr` tag must be empty for struct")
			}

			const FindTag = "find"
			const AttrTag = "attr"
			const TimeTag = "time"
			const ReTag = "re"

			vt := value.Type()
			for i := 0; i < vt.NumField(); i++ {
				fieldType := vt.Field(i)
				fieldValue := value.Field(i)

				selector := fieldType.Tag.Get(FindTag)
				selected := sel
				if selector != "" {
					selected = sel.Find(selector)
				}

				opt := UnmarshalOption{
					Attr: fieldType.Tag.Get(AttrTag),
					Time: fieldType.Tag.Get(TimeTag),
					Re:   fieldType.Tag.Get(ReTag),
					Loc:  opt.Loc,
				}

				if fieldType.PkgPath != "" {
					return UnmarshalFieldError{
						fieldType.Name,
						UnmarshalUnexportedFieldError{},
					}
				}

				err := unmarshalValue(fieldValue, selected, opt)
				if err != nil {
					return UnmarshalFieldError{
						fieldType.Name,
						err,
					}
				}
			}
			return nil
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
				return err
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
				return err
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

// Unmarshal parses selection and stores to v.
// if v is a struct, each field may specify following tags.
//  * `find` tag with CSS selector to specify sub element.
//  * `attr` tag with attribute name to get a text. if this tag not exists, get a text from text element.
//  * `re` tag with regular expression, use only matched substring from a text.
//  * `time` tag with time format to parse for time.Time.
func Unmarshal(v interface{}, selection *goquery.Selection, opt UnmarshalOption) error {
	if opt.Loc == nil {
		opt.Loc = time.UTC
	}

	ht := reflect.TypeOf(v)
	if ht.Kind() != reflect.Ptr {
		return UnmarshalMustBePointerError{}
	}

	return unmarshalValue(reflect.ValueOf(v).Elem(), selection, opt)
}
