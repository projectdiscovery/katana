package output

import (
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	jsoniter "github.com/json-iterator/go"
	"github.com/logrusorgru/aurora"
	"github.com/projectdiscovery/gologger"
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
	Write(*Result, *http.Response) error
	WriteErr(*Error) error
}

// StandardWriter is an standard output writer structure
type StandardWriter struct {
	storeFields      []string
	fields           string
	json             bool
	verbose          bool
	aurora           aurora.Aurora
	outputFile       *fileWriter
	outputMutex      *sync.Mutex
	storeResponse    bool
	storeResponseDir string
	errorFile        *fileWriter
}

// New returns a new output writer instance
func New(options Options) (Writer, error) {
	writer := &StandardWriter{
		fields:           options.Fields,
		json:             options.JSON,
		verbose:          options.Verbose,
		aurora:           aurora.NewAurora(options.Colors),
		outputMutex:      &sync.Mutex{},
		storeResponse:    options.StoreResponse,
		storeResponseDir: options.StoreResponseDir,
	}

	err := validateCustomFieldNames()
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

// Write writes the event to file and/or screen.
func (w *StandardWriter) Write(event *Result, resp *http.Response) error {
	if event != nil {
		if len(w.storeFields) > 0 {
			storeFields(event, w.storeFields)
		}
		var data []byte
		var err error

		if w.json {
			data, err = w.formatJSON(event)
		} else {
			data, err = w.formatScreen(event)
		}
		if err != nil {
			return errorutil.NewWithTag("output", "could not format output").Wrap(err)
		}
		if len(data) == 0 {
			return nil
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
	}

	if w.storeResponse && resp != nil {
		if file, err := getResponseFile(w.storeResponseDir, resp.Request.URL.String()); err == nil {
			data, err := w.formatResponse(resp)
			if err != nil {
				return errorutil.NewWithTag("output", "could not store response").Wrap(err)
			}
			if err := updateIndex(w.storeResponseDir, resp); err != nil {
				return errorutil.NewWithTag("output", "could not store response").Wrap(err)
			}
			if err := file.Write(data); err != nil {
				return errorutil.NewWithTag("output", "could not store response").Wrap(err)
			}
			file.Close()
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
