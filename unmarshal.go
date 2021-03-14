package scraper

import (
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

type UnmarshalTypeError struct{}

func (err UnmarshalTypeError) Error() string {
	return "Unmarshal: v must be a pointer to the value"
}

type UnmarshalFieldError struct {
	Field string
	Err   error
}

func (err UnmarshalFieldError) Error() string {
	return fmt.Sprintf("%v: %v", err.Field, err.Err)
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
	Attr string         // if nonempty, extract attribute of element. otherwise, uses Text()
	Re   string         // Regular Expression. must contain one capture.
	Time string         // for time.Time only. parse with this format.
	Loc  *time.Location // time zone for parsing time.Time.
}

func unmarshalValue(value reflect.Value, sel *goquery.Selection, opt UnmarshalOption) error {
	if !value.CanSet() {
		return UnmarshalTypeError{}
	}

	if value.Kind() == reflect.Slice {
		rv := reflect.MakeSlice(value.Type(), sel.Length(), sel.Length())
		for i := 0; i < sel.Length(); i++ {
			err := unmarshalValue(rv.Index(i), sel.Eq(i), opt)
			if err != nil {
				return fmt.Errorf("#%d: %v", i, err)
			}
		}
		value.Set(rv)
		return nil
	}

	if value.Kind() == reflect.Ptr {
		if sel.Length() == 0 {
			value.Set(reflect.Zero(value.Type()))
			return nil
		}
		newValue := reflect.New(value.Type().Elem())
		value.Set(newValue)
		value = newValue.Elem()
	}

	if sel.Length() != 1 {
		return fmt.Errorf("length(%v) != 1", sel.Length())
	}

	// extract text from Attr(attr) or Text()
	var s string
	if opt.Attr != "" {
		if w, ok := sel.Attr(opt.Attr); ok {
			s = w
		}
	} else {
		s = sel.Text()
	}

	// 正規表現パターンがあったら適用する
	if opt.Re != "" {
		re, err := regexp.Compile(opt.Re)
		if err != nil {
			return fmt.Errorf("re:%#v: %v", opt.Re, err)
		}
		submatch := re.FindStringSubmatch(s)
		n := len(submatch) - 1
		if n != 1 {
			return fmt.Errorf("re:%#v: matched count of the regular expression is %d, should be 1", opt.Re, n)
		}
		s = submatch[1]
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
			return fmt.Errorf("time tag must be empty unless time.Time")
		}
		if !value.CanAddr() {
			return fmt.Errorf("failed CanAddr: %v, %v", value, value.Type())
		}

		// その型が Unmarshaller を実装しているならそれを呼ぶ
		if inf, ok := value.Addr().Interface().(Unmarshaller); ok {
			return inf.Unmarshal(s)
		}

		if value.Kind() == reflect.Struct {
			const FindTag = "find"
			const AttrTag = "attr"
			const TimeTag = "time"
			const ReTag = "re"

			vt := value.Type()
			for i := 0; i < vt.NumField(); i++ {
				fieldType := vt.Field(i)
				fieldValue := value.Field(i)

				selector := fieldType.Tag.Get(FindTag)
				if selector == "" {
					return UnmarshalFieldError{
						fieldType.Name,
						fmt.Errorf("tag %v is required", FindTag),
					}
				}
				sel := sel.Find(selector)

				opt := UnmarshalOption{
					Attr: fieldType.Tag.Get(AttrTag),
					Time: fieldType.Tag.Get(TimeTag),
					Re:   fieldType.Tag.Get(ReTag),
					Loc:  opt.Loc,
				}

				err := unmarshalValue(fieldValue, sel, opt)
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

		case int, int32, int64:
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

func Unmarshal(v interface{}, selection *goquery.Selection, opt UnmarshalOption) error {
	if opt.Loc == nil {
		opt.Loc = time.UTC
	}

	ht := reflect.TypeOf(v)
	if ht.Kind() != reflect.Ptr {
		return UnmarshalTypeError{}
	}

	return unmarshalValue(reflect.ValueOf(v).Elem(), selection, opt)
}
