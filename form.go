package scraper

import (
	"bytes"
	"fmt"
	"golang.org/x/text/transform"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

type FormElementNotFoundError struct {
	Name string
}

func (error FormElementNotFoundError) Error() string {
	return fmt.Sprintf("form element %v not found", error.Name)
}

// AvailableValue holds an available value and corresponding label to display.
type AvailableValue struct {
	Value string
	Label string
}

// FormElement holds a form element.
type FormElement struct {
	Type            string // "select", "hidden", "submit", "text", "email", "password", "button", "checkbox", "radio", "image"
	Name            string
	Value           *AvailableValue
	AvailableValues []*AvailableValue
}

func (element *FormElement) AddAvailableValue(val *AvailableValue) {
	if element.AvailableValues == nil {
		element.AvailableValues = make([]*AvailableValue, 0)
	}
	element.AvailableValues = append(element.AvailableValues, val)
}

func (element *FormElement) GoString() string {
	buf := &bytes.Buffer{}
	_, _ = fmt.Fprintf(buf, "FormElement{\n")
	_, _ = fmt.Fprintf(buf, "  Type: %v\n", element.Type)
	_, _ = fmt.Fprintf(buf, "  Name: %v\n", element.Name)
	_, _ = fmt.Fprintf(buf, "  Value: %#v\n", element.Value)
	if element.AvailableValues != nil {
		_, _ = fmt.Fprintf(buf, "  AvailableValues: [")
		for _, a := range element.AvailableValues {
			_, _ = fmt.Fprintf(buf, "%#v, ", a)
		}
		_, _ = fmt.Fprintf(buf, "]\n")
	}
	_, _ = fmt.Fprintf(buf, "}\n")
	return buf.String()
}

// Form holds form data and submit information
type Form struct {
	url      *url.URL
	baseUrl  *url.URL
	Action   string
	Method   string
	Elements map[string]*FormElement
	Logger   Logger
}

// Form generates a Form object from a form object identified by selector in the Page
func (page *Page) Form(selector string) (*Form, error) {
	form := page.Find(selector)
	numForm := form.Length()
	if numForm != 1 {
		return nil, fmt.Errorf("selector='%v', found %v items. something went wrong", selector, numForm)
	}

	elements := map[string]*FormElement{}
	inputs := form.Find("input")
	for i := 0; i < inputs.Length(); i++ {
		s := inputs.Eq(i)
		name, ok := s.Attr("name")
		if !ok {
			t, ok := s.Attr("type")
			if !ok || t != "submit" {
				page.Logger.Printf("an input element without name (%#V) found. ignore.", s)
			}
			continue
		}
		t, ok := s.Attr("type")
		if !ok {
			t = "text"
		}
		value, ok := s.Attr("value")
		if !ok && strings.ToLower(t) == "radio" {
			value = "on"
		}
		element, ok := elements[name]
		if !ok {
			element = &FormElement{
				Type: t,
				Name: name,
			}
			elements[name] = element
		}

		val := &AvailableValue{
			Value: value,
		}
		if id, ok := s.Attr("id"); ok {
			idEscaped := strings.Replace(id, ".", "\\002e", -1)
			idEscaped = strings.Replace(idEscaped, ":", "\\003a", -1)
			//session.Printf("id %v -> %v\n", id, idEscaped)
			label := page.Find(fmt.Sprintf("label[for=%s]", idEscaped))
			if label.Length() > 0 {
				val.Label = label.Text()
			}
		}

		switch strings.ToLower(t) {
		case "submit", "hidden", "button", "text", "email", "password", "image":
			element.Value = val

		case "checkbox":
			element.AvailableValues = []*AvailableValue{val}
			if _, checked := s.Attr("checked"); checked {
				element.Value = val
			}

		case "radio":
			element.AddAvailableValue(val)
			if _, ok := s.Attr("checked"); ok {
				element.Value = val
			} else if element.Value == nil {
				element.Value = val // select first item by default
			}
		}
	}

	selects := form.Find("select")
	for i := 0; i < selects.Length(); i++ {
		s := selects.Eq(i)

		name, ok := s.Attr("name")
		if !ok {
			continue
		}

		element, ok := elements[name]
		if !ok {
			element = &FormElement{
				Type: "select",
				Name: name,
			}
			elements[name] = element
		}
		options := s.Find("option")
		for j := 0; j < options.Length(); j++ {
			o := options.Eq(j)

			value, ok := o.Attr("value")
			if !ok {
				// ignore an option without value
				continue
			}
			val := &AvailableValue{
				Value: value,
				Label: o.Text(),
			}
			element.AddAvailableValue(val)

			if _, ok := o.Attr("selected"); ok {
				element.Value = val
			}
			if element.Value == nil {
				element.Value = val // select first item by default
			}
		}
	}

	action, _ := form.Attr("action")

	method, ok := form.Attr("method")
	if !ok {
		method = "get"
	}
	return &Form{
		url:      page.Url,
		baseUrl:  page.BaseUrl,
		Action:   action,
		Method:   method,
		Elements: elements,
		Logger:   page.Logger,
	}, nil
}

// Set sets a value to the element specified by name.
// if element have AvailableValues(eg. check, radio or select elements), value must be equals one of them.
func (form *Form) Set(name string, value string) error {
	element, ok := form.Elements[name]
	if !ok {
		return FormElementNotFoundError{name}
	}
	if element.AvailableValues == nil {
		element.Value = &AvailableValue{Value: value}
	} else {
		var found *AvailableValue
		for _, val := range element.AvailableValues {
			if val.Value == value {
				found = val
				break
			}
		}
		if found == nil {
			var availableOptions []string
			for _, val := range element.AvailableValues {
				availableOptions = append(availableOptions, fmt.Sprintf("Value:%q Label:%q", val.Value, val.Label))
			}
			return fmt.Errorf("value %v is not available in [%s]", value, strings.Join(availableOptions, ", "))
		}
		element.Value = found
	}
	return nil
}

func (form *Form) SetForce(name string, value string) error {
	if _, ok := form.Elements[name]; ok {
		return form.Set(name, value)
	}
	form.Elements[name] = &FormElement{
		Type:  "hidden",
		Name:  name,
		Value: &AvailableValue{Value: value},
	}
	return nil
}

// find a Value from element name and its label of available values.
func (form *Form) ValueByLabel(name string, label string) (string, error) {
	element, ok := form.Elements[name]
	if !ok {
		return "", FormElementNotFoundError{name}
	}
	if element.AvailableValues == nil {
		return "", fmt.Errorf("form element %v is not a selection", name)
	}
	for _, val := range element.AvailableValues {
		if val.Label == label {
			return val.Value, nil
		}
	}

	// デバッグ用
	_ = form.PrintSelection(name)

	return "", fmt.Errorf("label %#v is not found in form element %v", label, name)
}

func (form *Form) SetByLabel(name string, label string) error {
	value, err := form.ValueByLabel(name, label)
	if err != nil {
		return err
	}
	return form.Set(name, value)
}

// PrintSelection shows available values of the element specified by name.
func (form *Form) PrintSelection(name string) error {
	element, ok := form.Elements[name]
	if !ok {
		return FormElementNotFoundError{name}
	}

	form.Logger.Printf("%v ->\n", name)

	if element.AvailableValues == nil {
		if element.Value == nil {
			form.Logger.Printf(" nil\n")
		} else {
			form.Logger.Printf(" * %#v\n", element.Value.Label)
		}
		return nil
	}

	for _, value := range element.AvailableValues {
		mark := " "
		if element.Value == value {
			mark = "*"
		}
		form.Logger.Printf(" %v %#v (%#v)\n", mark, value.Label, value.Value)
	}
	return nil
}

// Unset unset or uncheck the element specified by name.
func (form *Form) Unset(name string) error {
	element, ok := form.Elements[name]
	if !ok {
		return FormElementNotFoundError{name}
	}
	element.Value = nil
	return nil
}

// Check checks the checkbox specified by name.
func (form *Form) Check(name string) error {
	return form.Select(name, 0)
}

// Uncheck unchecks the checkbox specified by name.
func (form *Form) Uncheck(name string) error {
	return form.Unset(name)
}

// NumSelect returns number of available values of the select element specified by name.
func (form *Form) NumSelect(name string) (int, error) {
	element, ok := form.Elements[name]
	if !ok {
		return 0, FormElementNotFoundError{name}
	}
	return len(element.AvailableValues), nil
}

// Select sets an answer to the select element specified by name.
func (form *Form) Select(name string, index int) error {
	element, ok := form.Elements[name]
	if !ok {
		return FormElementNotFoundError{name}
	}
	if index < 0 || index >= len(element.AvailableValues) {
		return fmt.Errorf("select out of range %v in %#v", index, element.AvailableValues)
	}
	element.Value = element.AvailableValues[index]
	return nil
}

// SubmitOpt submits a form.
func (session *Session) Submit(form *Form) (*Response, error) {
	return session.SubmitOpt(form, "")
}

// SubmitOpt submits a form.
// if imageId is non-empty, specifies "image" element to imitate clicking.
func (session *Session) SubmitOpt(form *Form, imageId string) (*Response, error) {
	m := map[string]string{}
	for name, element := range form.Elements {
		if element.Value != nil {
			if element.Type == "image" {
				if imageId == "" || element.Name == imageId {
					key := func(member string) string {
						if name == "" {
							return member
						}
						return name + "." + member
					}
					m[key("x")] = "0"
					m[key("y")] = "0"
				}
			} else {
				m[name] = element.Value.Value
			}
		}
	}

	if session.ShowFormPosting {
		form.Logger.Printf("Form Posting:{\n")
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			form.Logger.Printf(" %v=%v\n", k, m[k])
		}
		form.Logger.Printf("}\n")
	}

	if session.Encoding != nil {
		if session.ShowFormPosting {
			form.Logger.Printf("converting to %v...\n", session.Encoding)
		}
		encoder := session.Encoding.NewEncoder()
		for k, v := range m {
			m[k], _, _ = transform.String(encoder, v)
		}
	}

	data := url.Values{}
	for k, v := range m {
		data.Set(k, v)
	}

	reqUrl, _ := form.baseUrl.Parse(form.Action)
	encoded := data.Encode()
	req, _ := http.NewRequest(strings.ToUpper(form.Method), reqUrl.String(), bytes.NewBufferString(encoded))
	req.Header.Set("Content-type", "application/x-www-form-urlencoded")
	req.Header.Set("Referer", form.url.String())
	req.Header.Set("Content-length", strconv.Itoa(len(encoded)))
	//req.Header.Set("Origin", reqUrl.Scheme + "://" + reqUrl.Host)
	return session.invoke(req)
}
