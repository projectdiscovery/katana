package output

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/logrusorgru/aurora"
	"github.com/mitchellh/mapstructure"
	"github.com/projectdiscovery/dsl"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	errorutil "github.com/projectdiscovery/utils/errors"
)

const (
	storeFieldsDirectory = "katana_field"
	indexFile            = "index.txt"
	DefaultResponseDir   = "katana_response"
)

var (
	decolorizerRegex = regexp.MustCompile(`\x1B\[[0-9;]*[a-zA-Z]`)
)

// Writer is an interface which writes output to somewhere for katana events.
type Writer interface {
	// Close closes the output writer interface
	Close() error
	// Write writes the event to file and/or screen.
	Write(*Result) error
	WriteErr(*Error) error
}

// StandardWriter is an standard output writer structure
type StandardWriter struct {
	storeFields           []string
	fields                string
	json                  bool
	verbose               bool
	aurora                aurora.Aurora
	outputFile            *fileWriter
	outputMutex           *sync.Mutex
	storeResponse         bool
	storeResponseDir      string
	omitRaw               bool
	omitBody              bool
	errorFile             *fileWriter
	matchRegex            []*regexp.Regexp
	filterRegex           []*regexp.Regexp
	extensionValidator    *extensions.Validator
	outputMatchCondition  string
	outputFilterCondition string
}

// New returns a new output writer instance
func New(options Options) (Writer, error) {
	writer := &StandardWriter{
		fields:                options.Fields,
		json:                  options.JSON,
		verbose:               options.Verbose,
		aurora:                aurora.NewAurora(options.Colors),
		outputMutex:           &sync.Mutex{},
		storeResponse:         options.StoreResponse,
		storeResponseDir:      options.StoreResponseDir,
		omitRaw:               options.OmitRaw,
		omitBody:              options.OmitBody,
		matchRegex:            options.MatchRegex,
		filterRegex:           options.FilterRegex,
		extensionValidator:    options.ExtensionValidator,
		outputMatchCondition:  options.OutputMatchCondition,
		outputFilterCondition: options.OutputFilterCondition,
	}
	// if fieldConfig empty get the default file
	if options.FieldConfig == "" {
		var err error
		options.FieldConfig, err = initCustomFieldConfigFile()
		if err != nil {
			return nil, err
		}
	}
	err := parseCustomFieldName(options.FieldConfig)
	if err != nil {
		return nil, err
	}
	err = loadCustomFields(options.FieldConfig, fmt.Sprintf("%s,%s", options.Fields, options.StoreFields))
	if err != nil {
		return nil, err
	}
	// Perform validations for fields and store-fields
	if options.Fields != "" {
		if err := validateFieldNames(options.Fields); err != nil {
			return nil, errorutil.NewWithTag("output", "could not validate fields").Wrap(err)
		}
	}
	if options.StoreFields != "" {
		_ = os.MkdirAll(storeFieldsDirectory, os.ModePerm)
		if err := validateFieldNames(options.StoreFields); err != nil {
			return nil, errorutil.NewWithTag("output", "could not validate store fields").Wrap(err)
		}
		writer.storeFields = append(writer.storeFields, strings.Split(options.StoreFields, ",")...)
	}
	if options.OutputFile != "" {
		output, err := newFileOutputWriter(options.OutputFile)
		if err != nil {
			return nil, errorutil.NewWithTag("output", "could not create output file").Wrap(err)
		}
		writer.outputFile = output
	}
	if options.StoreResponse {
		writer.storeResponseDir = DefaultResponseDir
		if options.StoreResponseDir != DefaultResponseDir && options.StoreResponseDir != "" {
			writer.storeResponseDir = options.StoreResponseDir
		}
		_ = os.RemoveAll(writer.storeResponseDir)
		_ = os.MkdirAll(writer.storeResponseDir, os.ModePerm)
		// todo: the index file seems never used?
		_, err := newFileOutputWriter(filepath.Join(writer.storeResponseDir, indexFile))
		if err != nil {
			return nil, errorutil.NewWithTag("output", "could not create index file").Wrap(err)
		}
	}
	if options.ErrorLogFile != "" {
		errorFile, err := newFileOutputWriter(options.ErrorLogFile)
		if err != nil {
			return nil, errorutil.NewWithTag("output", "could not create error file").Wrap(err)
		}

		writer.errorFile = errorFile
	}
	return writer, nil
}

// Write writes the result to file and/or screen.
func (w *StandardWriter) Write(result *Result) error {
	if result == nil {
		return errors.New("result is nil")
	}

	if len(w.storeFields) > 0 {
		storeFields(result, w.storeFields)
	}

	if !w.extensionValidator.ValidatePath(result.Request.URL) {
		return errors.New("result does not match extension filter")
	}

	if !w.matchOutput(result) {
		return errors.New("result does not match output")
	}
	if w.filterOutput(result) {
		return errors.New("result is filtered out")
	}
	var data []byte
	var err error

	if w.storeResponse && result.HasResponse() {
		if fileName, fileWriter, err := getResponseFile(w.storeResponseDir, result.Response.Resp.Request.URL.String()); err == nil {
			if absPath, err := filepath.Abs(fileName); err == nil {
				fileName = absPath
			}
			result.Response.StoredResponsePath = fileName
			data, err := w.formatResult(result)
			if err != nil {
				return errorutil.NewWithTag("output", "could not store response").Wrap(err)
			}
			if err := updateIndex(w.storeResponseDir, result); err != nil {
				return errorutil.NewWithTag("output", "could not store response").Wrap(err)
			}
			if err := fileWriter.Write(data); err != nil {
				return errorutil.NewWithTag("output", "could not store response").Wrap(err)
			}
			fileWriter.Close()
		}
	}

	if w.omitRaw {
		result.Request.Raw = ""
		if result.Response != nil {
			result.Response.Raw = ""
		}
	}
	if w.omitBody && result.HasResponse() {
		result.Response.Body = ""
	}

	if w.json {
		data, err = w.formatJSON(result)
	} else {
		data, err = w.formatScreen(result)
	}
	if err != nil {
		return errorutil.NewWithTag("output", "could not format output").Wrap(err)
	}
	if len(data) == 0 {
		return errors.New("result is empty")
	}
	w.outputMutex.Lock()
	defer w.outputMutex.Unlock()

	gologger.Silent().Msgf("%s", string(data))
	if w.outputFile != nil {
		if !w.json {
			data = decolorizerRegex.ReplaceAll(data, []byte(""))
		}
		if err := w.outputFile.Write(data); err != nil {
			return errorutil.NewWithTag("output", "could not write to output").Wrap(err)
		}
	}

	return nil
}

func (w *StandardWriter) WriteErr(errMessage *Error) error {
	data, err := jsoniter.Marshal(errMessage)
	if err != nil {
		return errorutil.NewWithTag("output", "marshal").Wrap(err)
	}
	if len(data) == 0 {
		return nil
	}
	w.outputMutex.Lock()
	defer w.outputMutex.Unlock()

	if w.errorFile != nil {
		if err := w.errorFile.Write(data); err != nil {
			return errorutil.NewWithTag("output", "write to error file").Wrap(err)
		}
	}
	return nil
}

// Close closes the output writer
func (w *StandardWriter) Close() error {
	if w.outputFile != nil {
		err := w.outputFile.Close()
		if err != nil {
			return err
		}
	}
	if w.errorFile != nil {
		err := w.errorFile.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// matchOutput checks if the event matches the output regex
func (w *StandardWriter) matchOutput(event *Result) bool {
	if w.matchRegex == nil && w.outputMatchCondition == "" {
		return true
	}

	for _, regex := range w.matchRegex {
		if regex.MatchString(event.Request.URL) {
			return true
		}
	}

	if w.outputMatchCondition != "" {
		return evalDslExpr(event, w.outputMatchCondition)
	}

	return false
}

// filterOutput returns true if the event should be filtered out
func (w *StandardWriter) filterOutput(event *Result) bool {
	if w.filterRegex == nil && w.outputFilterCondition == "" {
		return false
	}

	for _, regex := range w.filterRegex {
		if regex.MatchString(event.Request.URL) {
			return true
		}
	}

	if w.outputFilterCondition != "" {
		return evalDslExpr(event, w.outputFilterCondition)
	}

	return false
}

func evalDslExpr(result *Result, dslExpr string) bool {
	resultMap, err := resultToMap(*result)
	if err != nil {
		gologger.Warning().Msgf("Could not map result: %s\n", err)
		return false
	}

	res, err := dsl.EvalExpr(dslExpr, resultMap)
	if err != nil && !ignoreErr(err) {
		gologger.Error().Msgf("Could not evaluate DSL expression: %s\n", err)
		return false
	}
	return res == true
}

func resultToMap(result Result) (map[string]interface{}, error) {
	resultMap := make(map[string]any)
	config := &mapstructure.DecoderConfig{
		TagName: "json",
		Result:  &resultMap,
	}
	decoder, err := mapstructure.NewDecoder(config)
	if err != nil {
		return nil, fmt.Errorf("error creating decoder: %v", err)
	}
	err = decoder.Decode(result)
	if err != nil {
		return nil, fmt.Errorf("error decoding: %v", err)
	}
	return flatten(resultMap), nil
}

// mapsutil.Flatten w/o separator
func flatten(m map[string]any) map[string]any {
	o := make(map[string]any)
	for k, v := range m {
		switch child := v.(type) {
		case map[string]any:
			nm := flatten(child)
			for nk, nv := range nm {
				o[nk] = nv
			}
		default:
			o[k] = v
		}
	}
	return o
}

var (
	// showDSLErr controls whether to show hidden DSL errors or not
	showDSLErr = strings.EqualFold(os.Getenv("SHOW_DSL_ERRORS"), "true")
)

// ignoreErr checks if the error is to be ignored or not
// Reference: https://github.com/projectdiscovery/katana/pull/537
func ignoreErr(err error) bool {
	if showDSLErr {
		return false
	}
	if errors.Is(err, dsl.ErrParsingArg) || strings.Contains(err.Error(), "No parameter") {
		return true
	}
	return false
}
