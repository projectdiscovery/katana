package output

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFormatField(t *testing.T) {
	url := "https://policies.google.com/terms/file.php?hl=en-IN&fg=1"
	tests := []struct {
		url    string
		fields string
		result []string
	}{
		{url, "url", []string{url}},
		{url, "path", []string{"/terms/file.php"}},
		{url, "fqdn", []string{"policies.google.com"}},
		{url, "rdn", []string{"google.com"}},
		{url, "rurl", []string{"https://policies.google.com"}},
		{url, "file", []string{"file.php"}},
		{url, "key", []string{"hl", "fg"}},
		{url, "kv", []string{"hl=en-IN", "fg=1"}},
		{url, "value", []string{"en-IN", "1"}},
		{url, "dir", []string{"/terms/"}},
		{url, "udir", []string{"https://policies.google.com/terms/"}},
	}

	for _, test := range tests {
		result := formatField(&Result{URL: test.url}, test.fields)
		require.ElementsMatch(t, test.result, strings.Split(result, "\n"), "could not equal value")
	}
}
