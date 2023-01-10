package output

// Options contains the configuration options for output writer
type Options struct {
	Colors           bool
	JSON             bool
	Verbose          bool
	StoreResponse    bool
	OutputFile       string
	Fields           string
	StoreFields      string
	StoreResponseDir string
	FieldConfig      string
	ErrorLogFile     string
}
