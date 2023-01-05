package output

import (
	"bytes"
	"fmt"
)

// formatScreen formats the output for showing on screen.
func (w *StandardWriter) formatScreen(output *Result) ([]byte, error) {
	builder := &bytes.Buffer{}
	if w.fields != "" {
		result := formatField(output, w.fields)
		for _, fop := range result {
			if w.verbose {
				builder.WriteRune('[')
				builder.WriteString(w.aurora.Blue(fop.field).String())
				builder.WriteRune(']')
				builder.WriteRune(' ')
			}
			builder.WriteString(fmt.Sprintf("%s\n", fop.value))
		}
		return builder.Bytes(), nil
	}

	if w.verbose {
		builder.WriteRune('[')
		builder.WriteString(w.aurora.Blue(output.Tag).String())
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
