package output

import (
	"os"
	"regexp"

	"github.com/projectdiscovery/katana"
	"github.com/projectdiscovery/sliceutil"
	errorutil "github.com/projectdiscovery/utils/errors"
	stringsutil "github.com/projectdiscovery/utils/strings"
	"gopkg.in/yaml.v2"
)

// CustomFieldsMap is the global custom field data instance
// it is used for parsing the header and body of request
var CustomFieldsMap = make(map[string]katana.CustomFieldConfig)

type Part string

const (
	// RequestPart is the part of request
	Header   Part = "header"
	Body     Part = "body"
	Response Part = "response"
)

func parseCustomFieldName(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errorutil.NewWithTag("customfield", "could not read field config").Wrap(err)
	}
	defer file.Close()

	var data []katana.CustomFieldConfig
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return errorutil.NewWithTag("customfield", "could not decode field config").Wrap(err)
	}
	passedCustomFieldMap := make(map[string]katana.CustomFieldConfig)
	for _, item := range data {
		if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(item.Name) {
			return errorutil.New("wrong custom field name %s", item.Name)
		}
		// check custom field name is pre-defined or not
		if sliceutil.Contains(FieldNames, item.Name) {
			return errorutil.New("could not register custom field. \"%s\" already pre-defined field", item.Name)
		}
		// check custom field name should be unqiue
		if _, ok := passedCustomFieldMap[item.Name]; ok {
			return errorutil.New("could not register custom field. \"%s\" custom field already exists", item.Name)
		}
		passedCustomFieldMap[item.Name] = item
	}
	return nil
}

func loadCustomFields(filePath string, fields string) error {
	var err error

	file, err := os.Open(filePath)
	if err != nil {
		return errorutil.NewWithTag("customfield", "could not read field config").Wrap(err)
	}
	defer file.Close()

	var data []katana.CustomFieldConfig
	// read the field config file
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return errorutil.NewWithTag("customfield", "could not decode field config").Wrap(err)
	}
	allCustomFields := make(map[string]katana.CustomFieldConfig)
	for _, item := range data {
		for _, rg := range item.Regex {
			regex, err := regexp.Compile(rg)
			if err != nil {
				return errorutil.NewWithTag("customfield", "could not parse regex in field config").Wrap(err)
			}
			item.SetCompiledRegexp(regex)
		}
		if item.Part == "" {
			item.Part = Response.ToString()
		}
		allCustomFields[item.Name] = item
	}
	// Set the passed custom field value globally
	for _, f := range stringsutil.SplitAny(fields, ",") {
		if val, ok := allCustomFields[f]; ok {
			CustomFieldsMap[f] = val
		}
	}
	return nil
}

func (p Part) ToString() string {
	return string(p)
}
