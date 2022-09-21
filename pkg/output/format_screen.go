package output

import (
	"bytes"
	"fmt"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// formatScreen formats the output for showing on screen.
func (w *StandardWriter) formatScreen(output *Result) ([]byte, error) {
	builder := &bytes.Buffer{}

	// If fields are specified, use to format it
	if w.fields != "" {
		w.formatFieldWriter(builder, output)
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

func (w *StandardWriter) formatFieldWriter(builder *bytes.Buffer, output *Result) {
	parsed, _ := url.Parse(output.URL)
	if parsed == nil {
		return
	}

	queryLen := len(parsed.Query())
	queryKeys := make([]string, 0, queryLen)
	queryValues := make([]string, 0, queryLen)
	if queryLen > 0 {
		for k, v := range parsed.Query() {
			queryKeys = append(queryKeys, k)
			queryValues = append(queryValues, v...)
		}
	}
	hostname := parsed.Hostname()
	etld, _ := publicsuffix.EffectiveTLDPlusOne(hostname)
	values := []string{
		"url", output.URL,
		"rurl", fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host),
		"path", parsed.Path,
		"query", parsed.RawQuery,
		"fqdn", hostname,
		"rdn", etld,
	}
	if parsed.Path != "/" {
		if strings.HasSuffix(parsed.Path, "/") {
			values = append(values, "dir", parsed.Path)
		} else {
			values = append(values, "file", path.Base(parsed.Path))
		}
	}
	if len(queryKeys) > 0 || len(queryValues) > 0 {
		values = append(values, "key", strings.Join(queryKeys, ","))
		values = append(values, "value", strings.Join(queryValues, ","))
	}
	replacer := strings.NewReplacer(values...)
	_, _ = replacer.WriteString(builder, w.fields)
}
