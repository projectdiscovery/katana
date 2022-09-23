package output

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// FieldNames is a list of supported field names
var FieldNames = []string{
	"url",
	"path",
	"fqdn",
	"rdn",
	"rurl",
	"file",
	"key",
	"kv",
	"value",
	"dir",
	"udir",
}

// formatField formats output results based on fields from fieldNames
func formatField(output *Result, fields string) string {
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
	replaced := replacer.Replace(fields)
	if replaced == fields {
		return ""
	}
	return replaced
}
