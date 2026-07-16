package output

import (
	"os"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/utils/errkit"
	fileutil "github.com/projectdiscovery/utils/file"
	sliceutil "github.com/projectdiscovery/utils/slice"
	"gopkg.in/yaml.v2"
)

// CustomFieldsMap is the global custom field data instance
// it is used for parsing the header and body of request.
// Access it through the exported helpers so it stays safe under concurrent
// crawler/output initialization (see issue #1698).
var (
	CustomFieldsMap = make(map[string]CustomFieldConfig)
	customFieldsMu  sync.RWMutex
)

// setCustomField registers or overwrites a custom field entry.
func setCustomField(cfg CustomFieldConfig) {
	customFieldsMu.Lock()
	defer customFieldsMu.Unlock()
	CustomFieldsMap[cfg.Name] = cfg
}

// hasCustomField reports whether a custom field with the given name is registered.
func hasCustomField(name string) bool {
	customFieldsMu.RLock()
	defer customFieldsMu.RUnlock()
	_, ok := CustomFieldsMap[name]
	return ok
}

// CustomFieldsSnapshot returns a copy of the registered custom fields, safe to
// range over while other goroutines register new fields.
func CustomFieldsSnapshot() []CustomFieldConfig {
	customFieldsMu.RLock()
	defer customFieldsMu.RUnlock()
	fields := make([]CustomFieldConfig, 0, len(CustomFieldsMap))
	for _, cfg := range CustomFieldsMap {
		fields = append(fields, cfg)
	}
	return fields
}

type Part string

const (
	// RequestPart is the part of request
	Header   Part = "header"
	Body     Part = "body"
	Response Part = "response"
)

// CustomFieldConfig contains suggestions for field filling
type CustomFieldConfig struct {
	Name         string           `yaml:"name,omitempty"`
	Type         string           `yaml:"type,omitempty"`
	Part         string           `yaml:"part,omitempty"`
	Group        int              `yaml:"group,omitempty"`
	Regex        []string         `yaml:"regex,omitempty"`
	CompileRegex []*regexp.Regexp `yaml:"-"`
}

var DefaultFieldConfigData = []CustomFieldConfig{
	{
		Name:  "email",
		Type:  "regex",
		Part:  Response.ToString(),
		Regex: []string{`([a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+\.[a-zA-Z0-9_-]+)`},
	},
}

func (c *CustomFieldConfig) SetCompiledRegexp(r *regexp.Regexp) {
	c.CompileRegex = append(c.CompileRegex, r)
}

func (c *CustomFieldConfig) GetName() string {
	return c.Name
}

func parseCustomFieldName(filePath string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return errkit.Wrap(err, "customfield: could not read field config")
	}
	defer func() {
		if err := file.Close(); err != nil {
			gologger.Error().Msgf("Error closing file: %v\n", err)
		}
	}()

	var data []CustomFieldConfig
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return errkit.Wrap(err, "customfield: could not decode field config")
	}
	passedCustomFieldMap := make(map[string]CustomFieldConfig)
	for _, item := range data {
		if !regexp.MustCompile(`^[A-Za-z0-9_-]+$`).MatchString(item.Name) {
			return errkit.Newf("customfield: wrong custom field name %s", item.Name)
		}
		// check custom field name is pre-defined or not
		if sliceutil.Contains(FieldNames, item.Name) {
			return errkit.Newf("customfield: could not register custom field. \"%s\" already pre-defined field", item.Name)
		}
		// check custom field name should be unique
		if _, ok := passedCustomFieldMap[item.Name]; ok {
			return errkit.Newf("customfield: could not register custom field. \"%s\" custom field already exists", item.Name)
		}
		passedCustomFieldMap[item.Name] = item
	}
	return nil
}

func loadCustomFields(filePath string, fields string) error {
	var err error

	file, err := os.Open(filePath)
	if err != nil {
		return errkit.Wrap(err, "customfield: could not read field config")
	}
	defer func() {
		if err := file.Close(); err != nil {
			gologger.Error().Msgf("Error closing file: %v\n", err)
		}
	}()

	var data []CustomFieldConfig
	// read the field config file
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return errkit.Wrap(err, "customfield: could not decode field config")
	}
	for _, item := range data {
		for _, rg := range item.Regex {
			regex, err := regexp.Compile(rg)
			if err != nil {
				return errkit.Wrap(err, "customfield: could not parse regex in field config")
			}
			item.SetCompiledRegexp(regex)
		}
		if item.Part == "" {
			item.Part = Response.ToString()
		}
		setCustomField(item)
	}
	return nil
}

func initCustomFieldConfigFile() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", errkit.Wrap(err, "customfield: could not get home directory")
	}
	defaultConfig := filepath.Join(homedir, ".config", "katana", "field-config.yaml")

	if fileutil.FileExists(defaultConfig) {
		return defaultConfig, nil
	}
	if err := os.MkdirAll(filepath.Dir(defaultConfig), 0775); err != nil {
		return "", err
	}
	customFieldConfig, err := os.Create(defaultConfig)
	if err != nil {
		return "", errkit.Wrap(err, "customfield: could not get home directory")
	}
	defer func() {
		if err := customFieldConfig.Close(); err != nil {
			gologger.Error().Msgf("Error closing custom field config: %v\n", err)
		}
	}()

	err = yaml.NewEncoder(customFieldConfig).Encode(DefaultFieldConfigData)
	if err != nil {
		return "", err
	}
	return defaultConfig, nil
}

func (p Part) ToString() string {
	return string(p)
}
