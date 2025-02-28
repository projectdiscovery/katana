package output

import (
	jsoniter "github.com/json-iterator/go"
)

// formatJSON formats the output for json based formatting
func (w *StandardWriter) formatJSON(output *Result) ([]byte, error) {
	// // NOTE(dwisiswant0): No special treatment for custom fields.
	// // Ref: https://github.com/projectdiscovery/katana/issues/1182
	// if len(output.Request.CustomFields) > 0 {
	// 	return nil, nil
	// }
	return jsoniter.Marshal(output)
}
