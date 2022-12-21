package output

import (
	"bytes"
)

// formatScreen formats the output for showing on screen.
func (w *StandardWriter) formatScreen(output *Result) ([]byte, error) {
	// If fields are specified, use to format it
	if w.fields != "" {
		result := formatField(output, w.fields)
		return []byte(result), nil
	}
	builder := &bytes.Buffer{}

	if w.verbose {
		builder.WriteRune('[')
		builder.WriteString(w.aurora.Blue(output.Tag).String())
		builder.WriteRune(']')
		builder.WriteRune(' ')
		builder.WriteRune('[')
		builder.WriteString(w.aurora.Green(output.Status).String())
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
	if w.verbose {
		builder.WriteRune(' ')
		builder.WriteRune('[')
		builder.WriteString(w.aurora.Blue("bodylen").String())
		builder.WriteString(":")
		builder.WriteString(w.aurora.Blue(output.Len).String())
		builder.WriteRune(']')
		builder.WriteString(" [")
		builder.WriteString(w.aurora.Blue("Source").String())
		builder.WriteString(":")
		builder.WriteString(output.Source)
		builder.WriteRune(']')
	}

	if output.Body != "" && w.verbose {
		builder.WriteRune(' ')
		builder.WriteRune('[')
		builder.WriteString(output.Body)
		builder.WriteRune(']')
	}
	return builder.Bytes(), nil
}
