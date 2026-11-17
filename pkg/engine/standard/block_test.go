package standard

import (
	"fmt"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestMakeRequestKeepsRepeatedBlock(t *testing.T) {
	const challenge = "<html>Just a moment</html>"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cf-Mitigated", "challenge")
		w.WriteHeader(http.StatusForbidden)
		_, _ = fmt.Fprint(w, challenge)
	}))
	t.Cleanup(srv.Close)

	options, err := types.NewCrawlerOptions(&types.Options{
		MaxDepth:     1,
		FieldScope:   "rdn",
		BodyReadSize: math.MaxInt,
		Timeout:      1,
		Retries:      0,
		Concurrency:  1,
		Strategy:     "breadth-first",
		Silent:       true,
		OnResult:     func(output.Result) {},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = options.Close() })

	crawler, err := New(options)
	require.NoError(t, err)
	session, err := crawler.NewCrawlSessionWithURL(srv.URL)
	require.NoError(t, err)
	t.Cleanup(session.CancelFunc)

	for _, path := range []string{"/a", "/b"} {
		resp, err := crawler.makeRequest(session, &navigation.Request{Method: http.MethodGet, URL: srv.URL + path})
		require.NoError(t, err)
		require.NotNil(t, resp.Resp, "a repeated challenge body still identifies the blocked host")
		require.Equal(t, http.StatusForbidden, resp.StatusCode)
		require.Equal(t, challenge, resp.Body, "the challenge body stays in the output")
	}
}
