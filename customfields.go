package katana

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"regexp"

	"github.com/projectdiscovery/fileutil"
	"github.com/projectdiscovery/gologger"
	errorutil "github.com/projectdiscovery/utils/errors"

	"github.com/projectdiscovery/katana/pkg/utils"
	"gopkg.in/yaml.v2"
)

//go:embed field-config.yaml
var FieldConfig []byte

var AllCustomFieldsMap = make(map[string]CustomFieldConfig)

// CustomFieldConfig contains suggestions for field filling
type CustomFieldConfig struct {
	Name         string           `yaml:"name,omitempty"`
	Type         string           `yaml:"type,omitempty"`
	Part         string           `yaml:"part,omitempty"`
	Group        int              `yaml:"group,omitempty"`
	Regex        []string         `yaml:"regex,omitempty"`
	CompileRegex []*regexp.Regexp `yaml:"-"`
}

type Part string

const (
	// RequestPart is the part of request
	Header   Part = "header"
	Body     Part = "body"
	Response Part = "response"
)

func (c *CustomFieldConfig) SetCompiledRegexp(r *regexp.Regexp) {
	c.CompileRegex = append(c.CompileRegex, r)
}

func (c *CustomFieldConfig) GetName() string {
	return c.Name
}

func (p Part) ToString() string {
	return string(p)
}

func initCustomConfig(defaultConfig string) error {
	if fileutil.FileExists(defaultConfig) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(defaultConfig), 0775); err != nil {
		return err
	}
	err := os.WriteFile(defaultConfig, FieldConfig, 0644)
	if err != nil {
		return fmt.Errorf("could not create %v", &defaultConfig)
	}
	return nil

}

func LoadCustomFields(filePath string) error {
	var err error

	file, err := os.Open(filePath)
	if err != nil {
		return errorutil.NewWithTag("customfield", "could not read field config").Wrap(err)
	}
	defer file.Close()

	var data []CustomFieldConfig
	// read the field config file
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return errorutil.NewWithTag("customfield", "could not decode field config").Wrap(err)
	}
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
		AllCustomFieldsMap[item.Name] = item
	}
	return nil
}

func init() {
	defaultConfig, err := utils.GetDefaultCustomConfigFile()
	if err != nil {
		gologger.Error().Msg(err.Error())
		return
	}
	err = initCustomConfig(defaultConfig)
	if err != nil {
		gologger.Error().Label("customfield").Msg(err.Error())
		return
	}
	err = LoadCustomFields(defaultConfig)
	if err != nil {
		gologger.Error().Label("customfield").Msg(err.Error())
	}
}
