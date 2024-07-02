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

// FormOption is an option for a select input
type FormOption struct {
	Value      string
	Selected   string
	Attributes mapsutil.OrderedMap[string, string]
}

// FormSelect is a select input for a form field
type FormSelect struct {
	Name        string
	Attributes  mapsutil.OrderedMap[string, string]
	FormOptions []FormOption
}

type FormTextArea struct {
	Name       string
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

// FormSelectFill fills a map with selected values from a slice of FormSelect structs.
// It iterates over each FormSelect struct in the inputs slice and checks for a selected option.
// If a selected option is found, it adds the corresponding value to the map using the input's name as the key.
// If no option is selected, it selects the first option and adds its value to the map.
// The function returns the filled map.
func FormSelectFill(inputs []FormSelect) mapsutil.OrderedMap[string, string] {
	data := mapsutil.NewOrderedMap[string, string]()
	for _, input := range inputs {
		for _, option := range input.FormOptions {
			if option.Selected != "" {
				data.Set(input.Name, option.Value)
				break
			}
		}

		// If no option is selected, select the first one
		if !data.Has(input.Name) && len(input.FormOptions) > 0 {
			data.Set(input.Name, input.FormOptions[0].Value)
		}
	}
	return data
}

// FormTextAreaFill fills the form text areas with placeholder values.
// It takes a slice of FormTextArea structs as input and returns an OrderedMap
// containing the form field names as keys and the placeholder values as values.
func FormTextAreaFill(inputs []FormTextArea) mapsutil.OrderedMap[string, string] {
	data := mapsutil.NewOrderedMap[string, string]()
	for _, input := range inputs {
		data.Set(input.Name, FormData.Placeholder)
	}
	return data
}

// FormFillSuggestions takes a slice of form fields and returns an ordered map
// containing suggestions for filling those form fields. The function iterates
// over each form field and based on its type, calls the corresponding fill
// function to generate suggestions. The suggestions are then merged into a
// single ordered map and returned.
//
// Parameters:
// - formFields: A slice of form fields.
//
// Returns:
// An ordered map containing suggestions for filling the form fields.
func FormFillSuggestions(formFields []interface{}) mapsutil.OrderedMap[string, string] {
	merged := mapsutil.NewOrderedMap[string, string]()
	for _, item := range formFields {
		switch v := item.(type) {
		case FormInput:
			dataMapInputs := FormInputFillSuggestions([]FormInput{v})
			dataMapInputs.Iterate(func(key, value string) bool {
				merged.Set(key, value)
				return true
			})
		case FormSelect:
			dataMapSelects := FormSelectFill([]FormSelect{v})
			dataMapSelects.Iterate(func(key, value string) bool {
				merged.Set(key, value)
				return true
			})
		case FormTextArea:
			dataMapTextArea := FormTextAreaFill([]FormTextArea{v})
			dataMapTextArea.Iterate(func(key, value string) bool {
				merged.Set(key, value)
				return true
			})
		}
	}
	return merged
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

// ConvertGoquerySelectionToFormOption converts a goquery.Selection object to a FormOption object.
// It extracts the attributes from the goquery.Selection object and populates a FormOption object with the extracted values.
func ConvertGoquerySelectionToFormOption(item *goquery.Selection) FormOption {
	attrs := item.Nodes[0].Attr
	input := FormOption{Attributes: mapsutil.NewOrderedMap[string, string]()}
	for _, attribute := range attrs {
		switch attribute.Key {
		case "value":
			input.Value = attribute.Val

		case "selected":
			input.Selected = attribute.Key
		default:
			input.Attributes.Set(attribute.Key, attribute.Val)
		}
	}
	return input
}

// ConvertGoquerySelectionToFormSelect converts a goquery.Selection object to a FormSelect object.
// It extracts the attributes and form options from the goquery.Selection and populates them in the FormSelect object.
// The converted FormSelect object is then returned.
func ConvertGoquerySelectionToFormSelect(item *goquery.Selection) FormSelect {
	attrs := item.Nodes[0].Attr
	input := FormSelect{Attributes: mapsutil.NewOrderedMap[string, string]()}
	for _, attribute := range attrs {
		switch attribute.Key {
		case "name":
			input.Name = attribute.Val
		default:
			input.Attributes.Set(attribute.Key, attribute.Val)
		}
	}

	input.FormOptions = []FormOption{}
	item.Find("option").Each(func(_ int, option *goquery.Selection) {
		input.FormOptions = append(input.FormOptions, ConvertGoquerySelectionToFormOption(option))
	})
	return input
}

// ConvertGoquerySelectionToFormTextArea converts a goquery.Selection object to a FormTextArea struct.
// It extracts the attributes from the first node of the selection and populates a FormTextArea object with the extracted data.
// The "name" attribute is assigned to the Name field of the FormTextArea, while other attributes are added to the Attributes map.
func ConvertGoquerySelectionToFormTextArea(item *goquery.Selection) FormTextArea {
	attrs := item.Nodes[0].Attr
	input := FormTextArea{Attributes: mapsutil.NewOrderedMap[string, string]()}
	for _, attribute := range attrs {
		switch attribute.Key {
		case "name":
			input.Name = attribute.Val
		default:
			input.Attributes.Set(attribute.Key, attribute.Val)
		}
	}
	return input
}

// ConvertGoquerySelectionToFormField converts a goquery.Selection object to a form field.
// It checks the type of the selection and calls the appropriate conversion function.
// If the selection is an input, it calls ConvertGoquerySelectionToFormInput.
// If the selection is a select, it calls ConvertGoquerySelectionToFormSelect.
// If the selection is a textarea, it calls ConvertGoquerySelectionToFormTextArea.
// If the selection is of any other type, it returns nil.
func ConvertGoquerySelectionToFormField(item *goquery.Selection) interface{} {
	if item.Is("input") {
		return ConvertGoquerySelectionToFormInput(item)
	}
	if item.Is("select") {
		return ConvertGoquerySelectionToFormSelect(item)
	}
	if item.Is("textarea") {
		return ConvertGoquerySelectionToFormTextArea(item)
	}

	return nil
}
