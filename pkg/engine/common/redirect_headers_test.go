package common

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/stretchr/testify/require"
)

type headerRecorder struct {
	mu      sync.Mutex
	headers map[string]http.Header
}

func (r *headerRecorder) record(path string, h http.Header) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.headers == nil {
		r.headers = make(map[string]http.Header)
	}
	r.headers[path] = h.Clone()
}

func (r *headerRecorder) get(path string) http.Header {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.headers[path]
}

// TestRedirectCustomHeadersFollowScope ensures -H values are not sent to a
// redirect target outside the crawl scope, and are kept for in-scope hops.
func TestRedirectCustomHeadersFollowScope(t *testing.T) {
	rec := &headerRecorder{}

	// The out-of-scope host is [::1]; the crawl root is 127.0.0.1, so fqdn
	// scope keeps it out. An IP needs no name resolution.
	ln, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skipf("ipv6 loopback is not available: %v", err)
	}
	outside := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.record("outside", r.Header)
		_, _ = io.WriteString(w, "outside")
	}))
	_ = outside.Listener.Close()
	outside.Listener = ln
	outside.Start()
	defer outside.Close()
	outsideURL := outside.URL

	mux := http.NewServeMux()
	mux.HandleFunc("/away", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, outsideURL+"/collect", http.StatusFound)
	})
	mux.HandleFunc("/stay", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/final", http.StatusFound)
	})
	mux.HandleFunc("/final", func(w http.ResponseWriter, r *http.Request) {
		rec.record("final", r.Header)
		_, _ = io.WriteString(w, "final")
	})
	target := httptest.NewServer(mux)
	defer target.Close()

	dialer, err := fastdialer.NewDialer(fastdialer.DefaultOptions)
	require.NoError(t, err)
	defer dialer.Close()

	responseParser := parser.NewResponseParser()
	responseParser.InitWithOptions(&parser.Options{})

	scopeManager, err := scope.NewManager(nil, nil, "fqdn", false)
	require.NoError(t, err)

	shared := &Shared{
		Options: &types.CrawlerOptions{
			Options: &types.Options{
				MaxDepth:      10,
				Strategy:      "depth-first",
				Timeout:       5,
				BodyReadSize:  4096,
				CustomHeaders: []string{"X-Api-Key: APIKEY-456", "X-Session-Token: SESSION-789"},
			},
			OutputWriter:        &mockWriter{},
			UniqueFilter:        newMockFilter(),
			ScopeManager:        scopeManager,
			ExtensionsValidator: extensions.NewValidator(nil, nil, true),
			Dialer:              dialer,
			Parser:              responseParser,
		},
	}
	shared.Headers = shared.Options.Options.ParseCustomHeaders()

	session, err := shared.NewCrawlSessionWithURL(target.URL + "/")
	require.NoError(t, err)
	defer session.CancelFunc()

	do := func(path string) string {
		req, err := retryablehttp.NewRequest(http.MethodGet, target.URL+path, nil)
		require.NoError(t, err)
		for k, v := range shared.Headers {
			req.Header.Set(k, v)
		}
		resp, err := session.HttpClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck
		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return string(body)
	}

	require.Equal(t, "outside", do("/away"))
	got := rec.get("outside")
	require.NotNil(t, got)
	require.Empty(t, got.Get("X-Api-Key"), "custom header sent to out-of-scope redirect target")
	require.Empty(t, got.Get("X-Session-Token"), "custom header sent to out-of-scope redirect target")

	require.Equal(t, "final", do("/stay"))
	got = rec.get("final")
	require.NotNil(t, got)
	require.Equal(t, "APIKEY-456", got.Get("X-Api-Key"), "custom header dropped on in-scope redirect")
	require.Equal(t, "SESSION-789", got.Get("X-Session-Token"), "custom header dropped on in-scope redirect")
}

func TestRedirectHeadersAllowedWithoutScope(t *testing.T) {
	mustReq := func(raw string) *http.Request {
		req, err := http.NewRequest(http.MethodGet, raw, nil)
		require.NoError(t, err)
		return req
	}
	via := []*http.Request{mustReq("https://example.com/a")}

	require.True(t, redirectHeadersAllowed(mustReq("https://example.com:443/b").URL, via, nil))
	require.False(t, redirectHeadersAllowed(mustReq("https://other.example/b").URL, via, nil))
	require.False(t, redirectHeadersAllowed(mustReq("http://example.com/b").URL, via, nil))
	require.False(t, redirectHeadersAllowed(mustReq("https://example.com:8443/b").URL, via, nil))
}
