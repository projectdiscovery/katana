package utils

import (
	"fmt"
	"strconv"

	"github.com/PuerkitoBio/goquery"
	mapsutil "github.com/projectdiscovery/utils/maps"
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
	Email:       fmt.Sprintf("%s@example.org", xid.New().String()),
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
	Attributes mapsutil.OrderedMap[string, string]
}

// FormInputFillSuggestions returns a list of form filling suggestions
// for inputs returning the specified recommended values.
func FormInputFillSuggestions(inputs []FormInput) mapsutil.OrderedMap[string, string] {
	data := mapsutil.NewOrderedMap[string, string]()

	// Fill checkboxes and radioboxes first or default values first
	for _, input := range inputs {
		switch input.Type {
		case "radio":
			// Use a single radio name per value
			if !data.Has(input.Name) {
				data.Set(input.Name, input.Value)
			}
		case "checkbox":
			data.Set(input.Name, input.Value)

		default:
			// If there is a value, use it for the input. Else
			// infer the values based on input types.
			if input.Value != "" {
				data.Set(input.Name, input.Value)
			}
		}
	}

	// getIntWithdefault returns the integer value of the key or default value
	getIntWithdefault := func(input *FormInput, key string, defaultValue int) int {
		if value, ok := input.Attributes.Get(key); ok {
			if intValue, err := strconv.Atoi(value); err == nil {
				return intValue
			}
		}
		return defaultValue
	}

	// Fill rest of the inputs based on their types or name and ids
	for _, input := range inputs {
		if input.Value != "" {
			continue
		}
		switch input.Type {
		case "email":
			data.Set(input.Name, FormData.Email)
		case "color":
			data.Set(input.Name, FormData.Color)
		case "number", "range":
			min := getIntWithdefault(&input, "min", 1)
			max := getIntWithdefault(&input, "max", 10)
			step := getIntWithdefault(&input, "step", 1)
			val := min + step
			if val > max {
				val = max - step
			}
			data.Set(input.Name, strconv.Itoa(val))
		case "password":
			data.Set(input.Name, FormData.Password)
		case "tel":
			data.Set(input.Name, FormData.Password)
		default:
			data.Set(input.Name, FormData.Placeholder)
		}
	}
	return data
}

// ConvertGoquerySelectionToFormInput converts goquery selection to form input
func ConvertGoquerySelectionToFormInput(item *goquery.Selection) FormInput {
	attrs := item.Nodes[0].Attr
	input := FormInput{Attributes: mapsutil.NewOrderedMap[string, string]()}

	for _, attribute := range attrs {
		switch attribute.Key {
		case "name":
			input.Name = attribute.Val
		case "value":
			input.Value = attribute.Val
		case "type":
			input.Type = attribute.Val
		default:
			input.Attributes.Set(attribute.Key, attribute.Val)
		}
	}
	return input
}
