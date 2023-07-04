package output

import (
	"fmt"
	"net/url"
	"os"
	"path"
	"strings"

	"github.com/projectdiscovery/gologger"
	errorutil "github.com/projectdiscovery/utils/errors"
	stringsutil "github.com/projectdiscovery/utils/strings"
	urlutil "github.com/projectdiscovery/utils/url"
	"golang.org/x/net/publicsuffix"
)

// FieldNames is a list of supported field names
var FieldNames = []string{
	"url",
	"path",
	"fqdn",
	"rdn",
	"rurl",
	"qurl",
	"qpath",
	"file",
	"ufile",
	"key",
	"value",
	"kv",
	"dir",
	"udir",
}

type fieldOutput struct {
	field string
	value string
}

// validateFieldNames validates provided field names
func validateFieldNames(names string) error {
	parts := strings.Split(names, ",")
	if len(parts) == 0 {
		return errorutil.NewWithTag("customfield", "no field names provided: %s", names)
	}
	uniqueFields := make(map[string]struct{})
	for _, field := range FieldNames {
		uniqueFields[field] = struct{}{}
	}
	for _, field := range CustomFieldsMap {
		uniqueFields[field.Name] = struct{}{}
	}
	for _, part := range parts {
		if _, ok := uniqueFields[part]; !ok {
			return errorutil.NewWithTag("customfield", "invalid field %s specified: %s", part, names)
		}
	}
	return nil
}

// storeFields stores fields for a result into individual files
// based on name.
func storeFields(output *Result, storeFields []string) {
	parsed, err := urlutil.Parse(output.Request.URL)
	if err != nil {
		gologger.Warning().Msgf("storeFields: failed to parse url %v got %v", output.Request.URL, err)
		return
	}

	hostname := parsed.Hostname()
	etld, _ := publicsuffix.EffectiveTLDPlusOne(hostname)
	rootURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
	for _, field := range storeFields {
		if result := getValueForField(output, parsed.URL, hostname, etld, rootURL, field); result != "" {
			appendToFileField(parsed.URL, field, result)
		}
		if _, ok := CustomFieldsMap[field]; ok {
			results := getValueForCustomField(output)
			for _, result := range results {
				appendToFileField(parsed.URL, result.field, result.value)
			}
		}
	}
}

func appendToFileField(parsed *url.URL, field, data string) {
	file, err := os.OpenFile(path.Join(storeFieldsDirectory, fmt.Sprintf("%s_%s_%s.txt", parsed.Scheme, parsed.Hostname(), field)), os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		return
	}
	defer file.Close()

	_, _ = file.WriteString(data)
	_, _ = file.Write([]byte("\n"))
}

// formatField formats output results based on fields from fieldNames
func formatField(output *Result, fields string) []fieldOutput {
	var svalue []fieldOutput
	parsed, _ := urlutil.Parse(output.Request.URL)
	if parsed == nil {
		return svalue
	}

	queryBoth := []string{}
	queryKeys := []string{}
	queryValues := []string{}
	parsed.Query().Iterate(func(k string, v []string) bool {
		for _, value := range v {
			queryBoth = append(queryBoth, strings.Join([]string{k, value}, "="))
		}
		queryKeys = append(queryKeys, k)
		queryValues = append(queryValues, v...)
		return true
	})
	for _, f := range stringsutil.SplitAny(fields, ",") {
		switch f {
		case "url":
			svalue = append(svalue, fieldOutput{field: "url", value: output.Request.URL})
		case "rdn":
			hostname := parsed.Hostname()
			etld, _ := publicsuffix.EffectiveTLDPlusOne(hostname)
			svalue = append(svalue, fieldOutput{field: "rdn", value: etld})
		case "path":
			if parsed.Path != "" {
				svalue = append(svalue, fieldOutput{field: "path", value: parsed.Path})
			}
		case "fqdn":
			svalue = append(svalue, fieldOutput{field: "fqdn", value: parsed.Hostname()})
		case "rurl":
			svalue = append(svalue, fieldOutput{field: "rurl", value: fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)})
		case "qpath":
			if len(queryKeys) > 0 {
				svalue = append(svalue, fieldOutput{field: "qpath", value: fmt.Sprintf("%s?%s", parsed.Path, parsed.Query().Encode())})
			}
		case "qurl":
			if len(queryKeys) > 0 {
				svalue = append(svalue, fieldOutput{field: "qurl", value: output.Request.URL})
			}
		case "key":
			if len(queryKeys) > 0 || len(queryValues) > 0 || len(queryBoth) > 0 {
				for _, k := range queryKeys {
					svalue = append(svalue, fieldOutput{field: "key", value: k})
				}
			}
		case "kv":
			if len(queryKeys) > 0 || len(queryValues) > 0 || len(queryBoth) > 0 {
				for _, k := range queryBoth {
					svalue = append(svalue, fieldOutput{field: "kv", value: k})
				}
			}
		case "value":
			if len(queryKeys) > 0 || len(queryValues) > 0 || len(queryBoth) > 0 {
				for _, k := range queryValues {
					svalue = append(svalue, fieldOutput{field: "value", value: k})
				}
			}
		case "file":
			if parsed.Path != "" && parsed.Path != "/" {
				basePath := path.Base(parsed.Path)
				if strings.Contains(basePath, ".") {
					svalue = append(svalue, fieldOutput{field: "file", value: basePath})
				}
			}
		case "ufile":
			if parsed.Path != "" && parsed.Path != "/" {
				basePath := path.Base(parsed.Path)
				if strings.Contains(basePath, ".") {
					svalue = append(svalue, fieldOutput{field: "ufile", value: parsed.String()})
				}
			}
		case "udir":
			if parsed.Path != "" && parsed.Path != "/" {
				if strings.Contains(parsed.Path[1:], "/") {
					directory := parsed.Path[:strings.LastIndex(parsed.Path[1:], "/")+2]
					rootURL := fmt.Sprintf("%s://%s", parsed.Scheme, parsed.Host)
					svalue = append(svalue, fieldOutput{field: "udir", value: fmt.Sprintf("%s%s", rootURL, directory)})
				}
			}
		case "dir":
			if parsed.Path != "" && parsed.Path != "/" {
				if strings.Contains(parsed.Path[1:], "/") {
					directory := parsed.Path[:strings.LastIndex(parsed.Path[1:], "/")+2]
					svalue = append(svalue, fieldOutput{field: "dir", value: directory})
				}
			}
		default:
			for k, v := range output.Request.CustomFields {
				for _, r := range v {
					svalue = append(svalue, fieldOutput{field: k, value: r})
				}
			}
		}
	}
	return svalue
}

// getValueForField returns value for a field
func getValueForField(output *Result, parsed *url.URL, hostname, rdn, rurl, field string) string {
	switch field {
	case "url":
		return output.Request.URL
	case "path":
		return parsed.Path
	case "fqdn":
		return hostname
	case "rdn":
		return rdn
	case "rurl":
		return rurl
	case "ufile":
		basePath := path.Base(parsed.Path)
		if parsed.Path != "" && parsed.Path != "/" && strings.Contains(basePath, ".") {
			return parsed.String()
		}
	case "file":
		basePath := path.Base(parsed.Path)
		if parsed.Path != "" && parsed.Path != "/" && strings.Contains(basePath, ".") {
			return basePath
		}
	case "dir":
		if parsed.Path != "" && parsed.Path != "/" && strings.Contains(parsed.Path[1:], "/") {
			return parsed.Path[:strings.LastIndex(parsed.Path[1:], "/")+2]
		}
	case "udir":
		if parsed.Path != "" && parsed.Path != "/" && strings.Contains(parsed.Path[1:], "/") {
			return fmt.Sprintf("%s%s", rurl, parsed.Path[:strings.LastIndex(parsed.Path[1:], "/")+2])
		}
	case "qpath":
		if len(parsed.Query()) > 0 {
			return fmt.Sprintf("%s?%s", parsed.Path, parsed.Query().Encode())
		}
	case "qurl":
		if len(parsed.Query()) > 0 {
			return parsed.String()
		}
	case "key":
		values := make([]string, 0, len(parsed.Query()))
		for k := range parsed.Query() {
			values = append(values, k)
		}
		return strings.Join(values, "\n")
	case "value":
		values := make([]string, 0, len(parsed.Query()))
		for _, v := range parsed.Query() {
			values = append(values, v...)
		}
		return strings.Join(values, "\n")
	case "kv":
		values := make([]string, 0, len(parsed.Query()))
		for k, v := range parsed.Query() {
			for _, value := range v {
				values = append(values, strings.Join([]string{k, value}, "="))
			}
		}
		return strings.Join(values, "\n")
	}
	return ""
}

func getValueForCustomField(output *Result) []fieldOutput {
	var svalue []fieldOutput
	for k, v := range output.Request.CustomFields {
		for _, r := range v {
			svalue = append(svalue, fieldOutput{field: k, value: r})
		}
	}
	return svalue
}
