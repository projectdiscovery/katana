package output

import (
	jsoniter "github.com/json-iterator/go"
)

// formatJSON formats the output for json based formatting
func (w *StandardWriter) formatJSON(output *Result) ([]byte, error) {
	if len(output.Request.CustomFields) > 0 {
		return nil, nil
	}
	return jsoniter.Marshal(output)
}
