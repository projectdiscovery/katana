package utils

import (
	"fmt"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	"github.com/rs/xid"
)

// FormData is the global form fill data instance
var FormData FormFillData

func init() {
	FormData = DefaultFormFillData
}

// FormFillData contains suggestions for form filling
type FormFillData struct {
	Email       string `yaml:"email"`
	Color       string `yaml:"color"`
	Password    string `yaml:"password"`
	PhoneNumber string `yaml:"phone"`
	Placeholder string `yaml:"placeholder"`
}

var DefaultFormFillData = FormFillData{
	Email:       fmt.Sprintf("%s@katanacrawler.io", xid.New().String()),
	Color:       "#e66465",
	Password:    "katanaP@assw0rd1",
	PhoneNumber: "2124567890",
	Placeholder: "katana",
}

// FormInput is an input for a form field
type FormInput struct {
	Type       string
	Name       string
	Value      string
	Attributes map[string]string
}

// FormInputFillSuggestions returns a list of form filling suggestions
// for inputs returning the specified recommended values.
func FormInputFillSuggestions(inputs []FormInput) map[string]string {
	data := make(map[string]string)

	// Fill checkboxes and radioboxes first or default values first
	for _, input := range inputs {
		switch input.Type {
		case "radio":
			// Use a single radio name per value
			if _, ok := data[input.Name]; !ok {
				data[input.Name] = input.Value
			}
		case "checkbox":
			data[input.Name] = input.Value

		default:
			// If there is a value, use it for the input. Else
			// infer the values based on input types.
			if input.Value != "" {
				data[input.Name] = input.Value
			}
		}
	}

	// Fill rest of the inputs based on their types or name and ids
	for _, input := range inputs {
		if input.Value != "" {
			continue
		}

		switch input.Type {
		case "email":
			data[input.Name] = FormData.Email
		case "color":
			data[input.Name] = FormData.Color
		case "number", "range":
			var err error
			var max, min, step, val int

			if min, err = strconv.Atoi(input.Attributes["min"]); err != nil {
				min = 1
			}
			if max, err = strconv.Atoi(input.Attributes["max"]); err != nil {
				max = 10
			}
			if step, err = strconv.Atoi(input.Attributes["step"]); err != nil {
				step = 1
			}
			val = min + step
			if val > max {
				val = max - step
			}
			data[input.Name] = strconv.Itoa(val)
		case "password":
			data[input.Name] = FormData.Password
		case "tel":
			data[input.Name] = FormData.Password
		default:
			data[input.Name] = FormData.Placeholder
		}
	}
	return data
}

// ConvertGoquerySelectionToFormInput converts goquery selection to form input
func ConvertGoquerySelectionToFormInput(item *goquery.Selection) FormInput {
	attrs := item.Nodes[0].Attr
	input := FormInput{Attributes: make(map[string]string)}

	for _, attribute := range attrs {
		switch attribute.Key {
		case "name":
			input.Name = attribute.Val
		case "value":
			input.Value = attribute.Val
		case "type":
			input.Type = attribute.Val
		default:
			input.Attributes[attribute.Key] = attribute.Val
		}
	}
	return input
}
