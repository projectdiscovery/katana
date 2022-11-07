package output

import (
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/gologger"
)

// Writer is an interface which writes output to somewhere for katana events.
type Writer interface {
	// Close closes the output writer interface
	Close() error
	// Write writes the event to file and/or screen.
	Write(*Result) error
}

var decolorizerRegex = regexp.MustCompile(`\x1B\[[0-9;]*[a-zA-Z]`)

// StandardWriter is an standard output writer structure
type StandardWriter struct {
	storeFields []string
	fields      string
	json        bool
	verbose     bool
	aurora      aurora.Aurora
	outputFile  *fileWriter
	outputMutex *sync.Mutex
}

// Options contains the configuration options for output writer
type Options struct {
	// Color
	Colors bool
	// JSON specifies to write output in JSON format
	JSON string
	// OutputFile is the optional file to write output to
	OutputFile string
}

// Result is a result structure for the crawler
type Result struct {
	// Timestamp is the current timestamp
	Timestamp time.Time `json:"timestamp,omitempty"`
	// Method is the method for the result
	Method string `json:"method,omitempty"`
	// Body contains the body for the request
	Body string `json:"body,omitempty"`
	// URL is the URL of the result
	URL string `json:"endpoint,omitempty"`
	// Source is the source for the result
	Source string `json:"source,omitempty"`
	// Tag is the tag for the result
	Tag string `json:"tag,omitempty"`
	// Attribute is the attribute for the result
	Attribute string `json:"attribute,omitempty"`
}

const storeFieldsDirectory = "katana_output"

// New returns a new output writer instance
func New(colors, json, verbose bool, file, fields, storeFields string) (Writer, error) {
	writer := &StandardWriter{
		fields:      fields,
		json:        json,
		verbose:     verbose,
		aurora:      aurora.NewAurora(colors),
		outputMutex: &sync.Mutex{},
	}
	// Perform validations for fields and store-fields
	if fields != "" {
		if err := validateFieldNames(fields); err != nil {
			return nil, errors.Wrap(err, "could not validate fields")
		}
	}
	if storeFields != "" {
		_ = os.MkdirAll(storeFieldsDirectory, os.ModePerm)
		if err := validateFieldNames(storeFields); err != nil {
			return nil, errors.Wrap(err, "could not validate store fields")
		}
		writer.storeFields = append(writer.storeFields, strings.Split(storeFields, ",")...)
	}
	if file != "" {
		output, err := newFileOutputWriter(file)
		if err != nil {
			return nil, errors.Wrap(err, "could not create output file")
		}
		writer.outputFile = output
	}
	return writer, nil
}

// Write writes the event to file and/or screen.
func (w *StandardWriter) Write(event *Result) error {
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
		return errors.Wrap(err, "could not format output")
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
		if writeErr := w.outputFile.Write(data); writeErr != nil {
			return errors.Wrap(err, "could not write to output")
		}
	}
	return nil
}

// Close closes the output writer
func (w *StandardWriter) Close() error {
	var err error
	if w.outputFile != nil {
		err = w.outputFile.Close()
	}
	return err
}
