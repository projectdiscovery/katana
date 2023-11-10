package output

import (
	"regexp"

	"github.com/projectdiscovery/katana/pkg/utils/extensions"
)

// Options contains the configuration options for output writer
type Options struct {
	Colors                bool
	JSON                  bool
	Verbose               bool
	StoreResponse         bool
	OmitRaw               bool
	OmitBody              bool
	OutputFile            string
	Fields                string
	StoreFields           string
	StoreResponseDir      string
	FieldConfig           string
	ErrorLogFile          string
	MatchRegex            []*regexp.Regexp
	FilterRegex           []*regexp.Regexp
	ExtensionValidator    *extensions.Validator
	OutputMatchCondition  string
	OutputFilterCondition string
}
