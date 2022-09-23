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
	// If fields are specified, use to format it
	if w.fields != "" {
		result := w.formatFieldWriter(output)
		return []byte(result), nil
	}
	builder := &bytes.Buffer{}

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

func (w *StandardWriter) formatFieldWriter(output *Result) string {
	parsed, _ := url.Parse(output.URL)
	if parsed == nil {
		return ""
	}

	queryLen := len(parsed.Query())
	queryBoth := make([]string, 0, queryLen)
	queryKeys := make([]string, 0, queryLen)
	queryValues := make([]string, 0, queryLen)
	if queryLen > 0 {
		for k, v := range parsed.Query() {
			for _, value := range v {
				queryBoth = append(queryBoth, strings.Join([]string{k, value}, "="))
			}
			queryKeys = append(queryKeys, k)
			queryValues = append(queryValues, v...)
		}
	}
	hostname := parsed.Hostname()
	etld, _ := publicsuffix.EffectiveTLDPlusOne(hostname)
	rootURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	values := []string{
		"url", output.URL,
		"path", parsed.Path,
		"fqdn", hostname,
		"rdn", etld,
		"rurl", rootURL,
	}
	if parsed.Path != "" && parsed.Path != "/" {
		basePath := path.Base(parsed.Path)
		if strings.Contains(basePath, ".") {
			values = append(values, "file", basePath)
		}
		if strings.Contains(parsed.Path[1:], "/") {
			directory := parsed.Path[:strings.LastIndex(parsed.Path[1:], "/")+2]
			values = append(values, "dir", directory)
			values = append(values, "udir", fmt.Sprintf("%s%s", rootURL, directory))
		}
	}
	if len(queryKeys) > 0 || len(queryValues) > 0 || len(queryBoth) > 0 {
		values = append(values, "key", strings.Join(queryKeys, "\n"))
		values = append(values, "kv", strings.Join(queryBoth, "\n"))
		values = append(values, "value", strings.Join(queryValues, "\n"))
	}
	replacer := strings.NewReplacer(values...)
	replaced := replacer.Replace(w.fields)
	if replaced == w.fields {
		return ""
	}
	return replaced
}
