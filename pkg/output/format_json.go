package output

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/projectdiscovery/utils/structs"
)

// formatJSON formats the output for json based formatting
func (w *StandardWriter) formatJSON(output *Result) ([]byte, error) {
	finalMap, err := structs.FilterStructToMap(*output, nil, w.excludeOutputFields)
	if err != nil {
		return nil, err
	}

	if _, ok := finalMap["request"]; ok && output.Request != nil {
		reqMap, err := structs.FilterStructToMap(*output.Request, nil, w.excludeOutputFields)
		if err != nil {
			return nil, err
		}
		if len(reqMap) > 0 {
			finalMap["request"] = reqMap
		} else {
			delete(finalMap, "request")
		}
	}

	if _, ok := finalMap["response"]; ok && output.Response != nil {
		respMap, err := structs.FilterStructToMap(*output.Response, nil, w.excludeOutputFields)
		if err != nil {
			return nil, err
		}
		if len(respMap) > 0 {
			finalMap["response"] = respMap
		} else {
			delete(finalMap, "response")
		}
	}

	return jsoniter.Marshal(finalMap)
}
