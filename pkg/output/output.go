package output

import (
	"os"
	"regexp"
	"sync"

	"github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
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
	json        bool
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
	// URL is the URL of the result
	URL string `json:"url,omitempty"`
	// Source is the source for the result
	Source string `json:"source,omitempty"`
}

// New returns a new output writer instance
func New(colors, json bool, file string) (Writer, error) {
	auroraColorizer := aurora.NewAurora(colors)

	var outputFile *fileWriter
	if file != "" {
		output, err := newFileOutputWriter(file)
		if err != nil {
			return nil, errors.Wrap(err, "could not create output file")
		}
		outputFile = output
	}
	writer := &StandardWriter{
		json:        json,
		aurora:      auroraColorizer,
		outputFile:  outputFile,
		outputMutex: &sync.Mutex{},
	}
	return writer, nil
}

// Write writes the event to file and/or screen.
func (w *StandardWriter) Write(event *Result) error {
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
	w.outputMutex.Lock()
	defer w.outputMutex.Unlock()

	_, _ = os.Stdout.Write(data)
	_, _ = os.Stdout.Write([]byte("\n"))
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
