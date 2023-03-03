package customfield

import (
	"os"
	"path/filepath"

	"github.com/projectdiscovery/fileutil"
	errorutil "github.com/projectdiscovery/utils/errors"
)

// Part is the part of response
type Part int8

const (
	Header Part = iota
	Body
	Response
)

func (p Part) ToString() string {
	return []string{"header", "body", "response"}[p]
}

func GetDefaultCustomConfigFile() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", errorutil.NewWithTag("customfield", "could not get home directory").Wrap(err)
	}
	defaultConfig := filepath.Join(homedir, ".config", "katana", "field-config.yaml")
	return defaultConfig, nil
}

func CreateDefaultFieldConfigIfNotExists(defaultConfig string, data []byte) error {
	if fileutil.FileExists(defaultConfig) {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(defaultConfig), 0775); err != nil {
		return errorutil.NewWithTag("customfield", "%v", err.Error())
	}
	err := os.WriteFile(defaultConfig, data, 0644)
	if err != nil {
		return errorutil.NewWithTag("customfield", "could not create %v", &defaultConfig)
	}
	return nil
}
