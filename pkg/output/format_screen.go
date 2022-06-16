package output

import "bytes"

// formatScreen formats the output for showing on screen.
func (w *StandardWriter) formatScreen(output *Result) ([]byte, error) {
	builder := &bytes.Buffer{}

	if w.verbose {
		builder.WriteRune('[')
		builder.WriteString(w.aurora.Blue(output.Source).String())
		builder.WriteRune(']')
		builder.WriteRune(' ')
	}
	if output.Method != "" && w.verbose {
		builder.WriteRune('[')
		builder.WriteString(w.aurora.Green(output.Method).String())
		builder.WriteRune(']')
		builder.WriteRune(' ')
	}
	builder.WriteString(output.URL)

	if output.Body != "" && w.verbose {
		builder.WriteRune(' ')
		builder.WriteRune('[')
		builder.WriteString(output.Body)
		builder.WriteRune(']')
	}
	return builder.Bytes(), nil
}
