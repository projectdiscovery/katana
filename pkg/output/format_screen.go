package output

import "bytes"

// formatScreen formats the output for showing on screen.
func (w *StandardWriter) formatScreen(output *Result) ([]byte, error) {
	builder := &bytes.Buffer{}
	builder.WriteRune('[')
	builder.WriteString(w.aurora.Blue(output.Source).String())
	builder.WriteRune(']')
	builder.WriteRune(' ')
	builder.WriteString(output.URL)
	return builder.Bytes(), nil
}
