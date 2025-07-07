package output

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/valyala/fasttemplate"
)

func (w *StandardWriter) formatTemplate(output *Result) ([]byte, error) {
	var fieldOutputs []fieldOutput
	fieldNames := strings.Join(FieldNames, ",")
	fieldOutputs = formatField(output, fieldNames)
	fieldOutputs = append(fieldOutputs, getValueForCustomField(output)...)

	fieldsMap := make(map[string]string)
	for _, fo := range fieldOutputs {
		fieldsMap[fo.field] = fo.value
	}

	errUnknownTag := errors.New("unknown tag")

	tagFn := fasttemplate.TagFunc(func(w io.Writer, tag string) (int, error) {
		value, ok := fieldsMap[tag]
		if !ok {
			return 0, fmt.Errorf("%w %q", errUnknownTag, tag)
		}
		return w.Write([]byte(value))
	})

	out, err := w.outputTemplate.ExecuteFuncStringWithErr(tagFn)
	if err != nil {
		if errors.Is(err, errUnknownTag) {
			// If there is an unknown tag, we just ignore it.
			return nil, nil
		}
		return nil, err
	}

	return []byte(out), nil
}
