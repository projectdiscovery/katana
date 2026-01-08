package output

import (
	jsoniter "github.com/json-iterator/go"
	"github.com/projectdiscovery/utils/structs"
)

// formatJSON formats the output for json based formatting
func (w *StandardWriter) formatJSON(output *Result) ([]byte, error) {
	finalOrdMap, err := structs.FilterStructToMap(*output, nil, w.excludeOutputFields)
	if err != nil {
		return nil, err
	}

	if _, ok := finalOrdMap.Get("request"); ok && output.Request != nil {
		reqOrdMap, err := structs.FilterStructToMap(*output.Request, nil, w.excludeOutputFields)
		if err != nil {
			return nil, err
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
			return nil, err
		}
		if respOrdMap.Len() > 0 {
			finalOrdMap.Set("response", respOrdMap)
		} else {
			finalOrdMap.Delete("response")
		}
	}

	return jsoniter.Marshal(finalOrdMap)
}
