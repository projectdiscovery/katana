package output

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/stretchr/testify/require"
)

func TestValidateFieldNames(t *testing.T) {
	err := validateFieldNames("fqdn")
	require.Nil(t, err, "got error with valid field")

	err = validateFieldNames("")
	require.Error(t, err, "got no error with blank field")

	err = validateFieldNames("invalid")
	require.Error(t, err, "got no error with invalid field")
}

func TestFormatField(t *testing.T) {
	url := "https://policies.google.com/terms/file.php?hl=en-IN&fg=1"
	tests := []struct {
		url    string
		fields string
		result []fieldOutput
	}{
		{url, "url", []fieldOutput{{"url", url}}},
		{url, "path", []fieldOutput{{"path", "/terms/file.php"}}},
		{url, "fqdn", []fieldOutput{{"fqdn", "policies.google.com"}}},
		{url, "rdn", []fieldOutput{{"rdn", "google.com"}}},
		{url, "rurl", []fieldOutput{{"rurl", "https://policies.google.com"}}},
		{url, "file", []fieldOutput{{"file", "file.php"}}},
		{url, "key", []fieldOutput{{"key", "hl"}, {"key", "fg"}}},
		{url, "kv", []fieldOutput{{"kv", "hl=en-IN"}, {"kv", "fg=1"}}},
		{url, "value", []fieldOutput{{"value", "en-IN"}, {"value", "1"}}},
		{url, "dir", []fieldOutput{{"dir", "/terms/"}}},
		{url, "udir", []fieldOutput{{"udir", "https://policies.google.com/terms/"}}},
	}

	for _, test := range tests {
		result := formatField(&Result{Request: &navigation.Request{URL: test.url}}, test.fields)
		require.ElementsMatch(t, test.result, result, "could not equal value")
	}
}
