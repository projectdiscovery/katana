package common

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/projectdiscovery/fastdialer/fastdialer"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/stretchr/testify/require"
)

func TestRedirect_CustomHeaders_KeptInScope(t *testing.T) {
	var targetReceivedKey string

	var ts *httptest.Server
	ts = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, ts.URL+"/target", http.StatusFound)
			return
		}
		targetReceivedKey = r.Header.Get("X-Api-Key")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	defer ts.Close()

	dialer, err := fastdialer.NewDialer(fastdialer.DefaultOptions)
	require.NoError(t, err)

	options := &types.Options{
		Timeout:       5,
		CustomHeaders: []string{"X-Api-Key: my-secret-key"},
	}

	// Scope validator that considers the server in-scope
	scopeValidator := func(u *url.URL) bool {
		return true
	}

	client, _, err := BuildHttpClient(dialer, options, nil, scopeValidator)
	require.NoError(t, err)

	req, err := retryablehttp.NewRequest("GET", ts.URL+"/redirect", nil)
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", "my-secret-key")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, "my-secret-key", targetReceivedKey, "in-scope redirect should keep custom headers")
}

func TestRedirect_CustomHeaders_RemovedOutOfScope(t *testing.T) {
	var targetReceivedKey string
	var targetReceivedCookie string

	// Target server (out-of-scope)
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetReceivedKey = r.Header.Get("X-Api-Key")
		targetReceivedCookie = r.Header.Get("Cookie")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("target ok"))
	}))
	defer targetServer.Close()

	// Initial server redirects to out-of-scope target
	initialServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetServer.URL, http.StatusFound)
	}))
	defer initialServer.Close()

	dialer, err := fastdialer.NewDialer(fastdialer.DefaultOptions)
	require.NoError(t, err)

	options := &types.Options{
		Timeout:       5,
		CustomHeaders: []string{"X-Api-Key: my-secret-key", "Cookie: session=secret123"},
	}

	initialHost := strings.TrimPrefix(initialServer.URL, "http://")

	// Scope validator that only allows initial server host:port, marking targetServer out-of-scope
	scopeValidator := func(u *url.URL) bool {
		return strings.EqualFold(u.Host, initialHost)
	}

	client, _, err := BuildHttpClient(dialer, options, nil, scopeValidator)
	require.NoError(t, err)

	req, err := retryablehttp.NewRequest("GET", initialServer.URL, nil)
	require.NoError(t, err)
	req.Header.Set("X-Api-Key", "my-secret-key")
	req.Header.Set("Cookie", "session=secret123")

	resp, err := client.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Empty(t, targetReceivedKey, "out-of-scope redirect should remove custom X-Api-Key header")
	require.Empty(t, targetReceivedCookie, "out-of-scope redirect should remove custom Cookie header")
}
