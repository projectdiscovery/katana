package output

import (
	"fmt"

	jsoniter "github.com/json-iterator/go"
	"github.com/projectdiscovery/utils/structs"
)

// formatJSON formats the output for json based formatting
func (w *StandardWriter) formatJSON(output *Result) ([]byte, error) {
	finalOrdMap, err := structs.FilterStructToMap(*output, nil, w.excludeOutputFields)
	if err != nil {
		return nil, fmt.Errorf("failed to filter struct: %w", err)
	}

	if _, ok := finalOrdMap.Get("request"); ok && output.Request != nil {
		reqOrdMap, err := structs.FilterStructToMap(*output.Request, nil, w.excludeOutputFields)
		if err != nil {
			return nil, fmt.Errorf("failed to filter request: %w", err)
		}
		if reqOrdMap.Len() > 0 {
			finalOrdMap.Set("request", reqOrdMap)
		} else {
			finalOrdMap.Delete("request")
		}
	}

	if _, ok := finalOrdMap.Get("response"); ok && output.Response != nil {
		respOrdMap, err := structs.FilterStructToMap(*output.Response, nil, w.excludeOutputFields)
		if err != nil {
			return nil, fmt.Errorf("failed to filter response: %w", err)
		}
		if respOrdMap.Len() > 0 {
			finalOrdMap.Set("response", respOrdMap)
		} else {
			finalOrdMap.Delete("response")
		}
	}

	// FIX: Sanitize the map to ensure all values are JSON serializable
	// This fixes issues when using -jsonl with -headless mode
	sanitized := sanitizeForJSON(finalOrdMap)

	return jsoniter.Marshal(sanitized)
}

// sanitizeForJSON recursively sanitizes a map to ensure all values are JSON serializable
// This prevents errors when headless mode produces non-serializable data
func sanitizeForJSON(v interface{}) interface{} {
	switch val := v.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = sanitizeForJSON(v)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = sanitizeForJSON(v)
		}
		return result
	case string, int, int64, float64, bool, nil:
		return val
	case []string:
		result := make([]interface{}, len(val))
		for i, v := range val {
			result[i] = v
		}
		return result
	case map[string]string:
		result := make(map[string]interface{})
		for k, v := range val {
			result[k] = v
		}
		return result
	default:
		// For any other types (functions, channels, complex structs), convert to string
		return fmt.Sprintf("%v", val)
	}
}
