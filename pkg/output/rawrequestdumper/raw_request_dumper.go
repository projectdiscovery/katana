package rawrequestdumper

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/output/filewriter"
)

// Dumper is an interface which writes raw request data to somewhere for katana requests.
type Dumper interface {
	// Close closes the output Dumper interface
	Close() error
	// Write writes the event to file and/or screen.
	Write(string) error
}

// RawReqDumper is a raw request dumper structure
type RawReqDumper struct {
	outputFile     *filewriter.Writer
	outputFileName string
	outputMutex    *sync.Mutex
}

// Options contains the configuration options for output dumper
type Options struct {
	// OutputFile is the optional file to dump output to
	OutputFile string
}

// New returns a new output RawReqDumper instance
func NewReqDumper(filename string) (*RawReqDumper, error) {
	dumper := &RawReqDumper{
		outputMutex: &sync.Mutex{},
	}
	if filename != "" {
		output, err := filewriter.NewFileOutputWriter(filename)
		dumper.outputFileName = ""
		if err != nil {
			return nil, errors.Wrap(err, "could not create output file")
		}
		dumper.outputFile = output
		dumper.outputFileName = filename
		dumper.StartFile()
	}
	return dumper, nil
}

// Dump dumps the event to yaml file
func (w *RawReqDumper) DumpToFile(data string) error {
	if len(data) == 0 {
		return nil
	}

	re := regexp.MustCompile(`\n`)
	data = "\n    - |-\n      " + re.ReplaceAllString(data, "\n      ")
	w.outputMutex.Lock()
	defer w.outputMutex.Unlock()

	if w.outputFile != nil {
		if writeErr := w.outputFile.Write([]byte(data)); writeErr != nil {
			return errors.Wrap(writeErr, "could not write data in rawreqdumper.DumpToFile")
		}
	}
	return nil
}

func (w *RawReqDumper) StartFile() error {
	if writeErr := w.outputFile.Write([]byte("requests:\n  - raw:")); writeErr != nil {
		fmt.Println("could not write data in rawreqdumper.StartFile")
		return errors.Wrap(writeErr, "could not write data in rawreqdumper.StartFile")
	}
	return nil
}

// DumpRawRequest prints to the screen an ascii representation of a request
func (w *RawReqDumper) Dump(header http.Header, body string, r *http.Request, toScreen bool) {
	req := BuildRawRequest(header, body, r)
	if w.outputFileName != "" {
		w.DumpToFile(req)
	}

	if toScreen {
		gologger.Silent().Msgf("%s\n", req)
	}
}

// From header object and request body data, build and return the raw request string
func BuildRawRequest(header http.Header, body string, r *http.Request) string {
	var request []string
	// get path from url
	var path string
	re := regexp.MustCompile(`^https?:\/\/[A-Za-z0-9:.]*([\/]{1}.*\/?)$`)
	match := re.FindStringSubmatch(r.URL.String())
	if path = "/"; len(match) > 1 {
		path = match[1]
	}
	// build request start line and add to request array
	start_line := fmt.Sprintf("%v %v %v", r.Method, path, r.Proto)
	request = append(request, start_line)
	// add Host header to request array
	request = append(request, fmt.Sprintf("Host: %v", r.Host))
	// add all other headers provided to request array
	for name, headers := range header {
		for _, h := range headers {
			request = append(request, fmt.Sprintf("%v: %v", name, h))
		}
	}

	// If this is a POST, add post data
	if r.Method == "POST" {
		// get content length of body
		content_length := len(body)
		// add condent-length request header to request array
		request = append(request, fmt.Sprintf("Content-length: %v", content_length))
		// add body to request array
		request = append(request, "\n"+body)
	} // Return the request as a string

	return strings.Join(request, "\n")
}

// Close closes the output writer
func (w *RawReqDumper) Close() error {
	var err error
	if w.outputFile != nil {
		err = w.outputFile.Close()
	}
	return err
}
