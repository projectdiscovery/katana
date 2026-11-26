package output

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/stretchr/testify/require"
)

// The crawler parses the same response after writing it, so omitting fields
// from output must not strip them from the caller's result.
func TestWriteOmitDoesNotMutateResult(t *testing.T) {
	outFile := filepath.Join(t.TempDir(), "out.jsonl")
	writer, err := New(Options{
		JSON:               true,
		OmitBody:           true,
		OmitRaw:            true,
		OutputFile:         outFile,
		ExtensionValidator: extensions.NewValidator(nil, nil, true),
	})
	require.NoError(t, err)

	result := &Result{
		Request: &navigation.Request{URL: "http://example.com/api/data.json", Raw: "GET /api/data.json"},
		Response: &navigation.Response{
			Resp: &http.Response{StatusCode: http.StatusOK},
			Body: `{"next":"/p/one.html"}`,
			Raw:  "HTTP/1.1 200 OK",
		},
	}
	require.NoError(t, writer.Write(result))
	require.NoError(t, writer.Close())

	require.Equal(t, `{"next":"/p/one.html"}`, result.Response.Body)
	require.Equal(t, "HTTP/1.1 200 OK", result.Response.Raw)
	require.Equal(t, "GET /api/data.json", result.Request.Raw)

	data, err := os.ReadFile(outFile)
	require.NoError(t, err)
	require.NotContains(t, string(data), `"body"`)
	require.NotContains(t, string(data), `"raw"`)
}
