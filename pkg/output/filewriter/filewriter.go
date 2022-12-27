package filewriter

import (
	"bufio"
	"os"
)

// fileWriter is a concurrent file based output writer.
type Writer struct {
	file   *os.File
	writer *bufio.Writer
}

// NewFileOutputWriter creates a new buffered writer for a file
func NewFileOutputWriter(file string) (*Writer, error) {
	output, err := os.Create(file)
	if err != nil {
		return nil, err
	}
	return &Writer{file: output, writer: bufio.NewWriter(output)}, nil
}

// WriteString writes an output to the underlying file
func (w *Writer) Write(data []byte) error {
	_, err := w.writer.Write(data)
	if err != nil {
		return err
	}
	_, err = w.writer.WriteRune('\n')
	return err
}

// Close closes the underlying writer flushing everything to disk
func (w *Writer) Close() error {
	w.writer.Flush()
	//nolint:errcheck // we don't care whether sync failed or succeeded.
	w.file.Sync()
	return w.file.Close()
}
