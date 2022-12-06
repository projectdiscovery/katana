package output

import (
	"fmt"
	"os"
	"regexp"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/sliceutil"
	stringsutil "github.com/projectdiscovery/utils/strings"
	"gopkg.in/yaml.v2"
)

// CustomFieldsMap is the global custom field data instance
// it is used for parsing the header and body of request
var CustomFieldsMap = make(map[string]CustomFieldConfig)

// FormFillData contains suggestions for form filling
type CustomFieldConfig struct {
	Name         string           `yaml:"name,omitempty"`
	Type         string           `yaml:"type,omitempty"`
	Group        int              `yaml:"group,omitempty"`
	Regex        []string         `yaml:"regex,omitempty"`
	CompileRegex []*regexp.Regexp `yaml:"-"`
}

var DefaultFieldConfigData = []CustomFieldConfig{
	{
		Name:  "email",
		Type:  "regex",
		Regex: []string{`([a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+\.[a-zA-Z0-9_-]+)`},
	},
}

func (c *CustomFieldConfig) SetCompiledRegexp(r *regexp.Regexp) {
	c.CompileRegex = append(c.CompileRegex, r)
}

func (c *CustomFieldConfig) GetName() string {
	return c.Name
}

func GetCustomFieldNames(filePath string) ([]string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, errors.Wrap(err, "could not read form config")
	}
	defer file.Close()

	var data []CustomFieldConfig
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return nil, errors.Wrap(err, "could not decode form config")
	}
	var result []string
	for _, item := range data {
		result = append(result, item.Name)
	}
	return result, nil
}

func ParseCustomFieldName(filePath string, fields string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errors.Wrap(err, "could not read form config")
	}
	defer file.Close()

	var data []CustomFieldConfig
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return errors.Wrap(err, "could not decode form config")
	}
	passedCustomFieldMap := make(map[string]CustomFieldConfig)
	for _, item := range data {
		if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(item.Name) {
			return fmt.Errorf("wrong custom field name %s", item.Name)
		}
		// check custom field name is pre-defined or not
		if sliceutil.Contains(FieldNames, item.Name) {
			return fmt.Errorf("could not register custom field. \"%s\" already pre-defined field", item.Name)
		}
		// check custom field name should be unqiue
		if _, ok := passedCustomFieldMap[item.Name]; ok {
			return fmt.Errorf("could not register custom field. \"%s\" custom field already exists", item.Name)
		}
		for _, rg := range item.Regex {
			item.SetCompiledRegexp(regexp.MustCompile(rg))
		}
		passedCustomFieldMap[item.Name] = item
	}

	// Set the passed custom field value globally
	for _, f := range stringsutil.SplitAny(fields, ",") {
		if val, ok := passedCustomFieldMap[f]; ok {
			CustomFieldsMap[f] = val
		}
	}
	return nil
}
