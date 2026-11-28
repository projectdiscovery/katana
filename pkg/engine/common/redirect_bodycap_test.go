package common

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/stretchr/testify/require"
)

// TestRedirectBodyCapped ensures the redirect callback honors BodyReadSize and
// does not read the whole redirect body. The intermediate 302 streams an
// effectively unbounded body, so an uncapped io.ReadAll would block inside the
// redirect callback until the client timeout fires; the cap lets the request
// follow through to the final response.
func TestRedirectBodyCapped(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/redirect", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "/final")
		w.WriteHeader(http.StatusFound)
		flusher, _ := w.(http.Flusher)
		chunk := bytes.Repeat([]byte("A"), 4096)
		for {
			select {
			case <-r.Context().Done():
				return
			default:
			}
			if _, err := w.Write(chunk); err != nil {
				return
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	})
	mux.HandleFunc("/final", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, "final-ok")
	})
	server := httptest.NewServer(mux)
	defer server.Close()

	dialer, err := fastdialer.NewDialer(fastdialer.DefaultOptions)
	require.NoError(t, err)
	defer dialer.Close()

	responseParser := parser.NewResponseParser()
	responseParser.InitWithOptions(&parser.Options{})

	scopeManager, err := scope.NewManager(nil, nil, "", true)
	require.NoError(t, err)

	shared := &Shared{
		Options: &types.CrawlerOptions{
			Options: &types.Options{
				MaxDepth:     10,
				Strategy:     "depth-first",
				Timeout:      5,
				BodyReadSize: 4096,
			},
			OutputWriter:        &mockWriter{},
			UniqueFilter:        newMockFilter(),
			ScopeManager:        scopeManager,
			ExtensionsValidator: extensions.NewValidator(nil, nil, true),
			Dialer:              dialer,
			Parser:              responseParser,
		},
	}

	session, err := shared.NewCrawlSessionWithURL(server.URL + "/redirect")
	require.NoError(t, err)
	defer session.CancelFunc()

	req, err := retryablehttp.NewRequest(http.MethodGet, server.URL+"/redirect", nil)
	require.NoError(t, err)

	type result struct {
		body string
		err  error
	}
	done := make(chan result, 1)
	go func() {
		resp, err := session.HttpClient.Do(req)
		if err != nil {
			done <- result{err: err}
			return
		}
		defer resp.Body.Close() //nolint:errcheck
		body, err := io.ReadAll(resp.Body)
		done <- result{body: string(body), err: err}
	}()

	select {
	case res := <-done:
		require.NoError(t, res.err, "request should follow redirect instead of blocking on unbounded body")
		require.Equal(t, "final-ok", res.body)
	case <-time.After(10 * time.Second):
		t.Fatal("redirect body was not capped: request blocked reading the unbounded redirect body")
	}
}
