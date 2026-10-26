package main

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRecorderRecordsWhatTheServerReceived(t *testing.T) {
	var gotBody string
	var gotHost string
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		gotBody, gotHost = string(body), r.Host
		if r.URL.Path == "/missing" {
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer app.Close()

	upstream, _ := url.Parse(app.URL)
	rec := newRecorder(upstream)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	resp, err := http.Post(proxy.URL+"/login?next=/home", "application/x-www-form-urlencoded", strings.NewReader("user=a&pass=b"))
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, "user=a&pass=b", gotBody, "body must still reach the app")
	require.Equal(t, strings.TrimPrefix(proxy.URL, "http://"), gotHost, "app must see the proxy host")

	resp, err = http.Get(proxy.URL + "/missing")
	require.NoError(t, err)
	_ = resp.Body.Close()

	hits := rec.snapshot()
	require.Len(t, hits, 2)
	require.Equal(t, "POST", hits[0].Method)
	require.True(t, hits[0].Body)
	require.ElementsMatch(t, []string{"next", "user", "pass"}, hits[0].Params)
	require.Equal(t, http.StatusNotFound, hits[1].Status)
}

func TestRecorderTreatsFallbackPageAsNotFound(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/items" {
			_, _ = w.Write([]byte(`[1,2]`))
			return
		}
		// single page apps answer every unknown path with their index page
		_, _ = w.Write([]byte("<html>app shell</html>"))
	}))
	defer app.Close()

	upstream, _ := url.Parse(app.URL)
	rec := newRecorder(upstream)
	require.NoError(t, rec.calibrate())
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	for _, p := range []string{"/api/items", "/made/up/path", "/another.js"} {
		resp, err := http.Get(proxy.URL + p)
		require.NoError(t, err)
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		require.NotEmpty(t, body, "client must still get the body")
	}
	require.Equal(t, 1, computeMetrics(rec.snapshot(), 0).Routes)
}

func TestRecorderUpgradesConnections(t *testing.T) {
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Connection", "Upgrade")
		w.Header().Set("Upgrade", "websocket")
		w.WriteHeader(http.StatusSwitchingProtocols)
	}))
	defer app.Close()

	upstream, _ := url.Parse(app.URL)
	rec := newRecorder(upstream)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	req, _ := http.NewRequest(http.MethodGet, proxy.URL+"/socket.io/", nil)
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Upgrade", "websocket")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()
	require.Equal(t, http.StatusSwitchingProtocols, resp.StatusCode)
	require.Equal(t, http.StatusSwitchingProtocols, rec.snapshot()[0].Status)
}

func TestRecorderDoesNotCountAbandonedRequestsAsErrors(t *testing.T) {
	release := make(chan struct{})
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-release }))
	defer app.Close()
	defer close(release)

	upstream, _ := url.Parse(app.URL)
	rec := newRecorder(upstream)
	proxy := httptest.NewServer(rec)
	defer proxy.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, proxy.URL+"/slow", nil)
	_, err := http.DefaultClient.Do(req)
	require.Error(t, err)

	require.Eventually(t, func() bool { return len(rec.snapshot()) == 1 }, 2*time.Second, 20*time.Millisecond)
	require.Zero(t, computeMetrics(rec.snapshot(), 0).Errors)
}

func TestBodyKeys(t *testing.T) {
	require.Equal(t, []string{"email", "password"}, bodyKeys("application/json", []byte(`{"password":"x","email":"y"}`)))

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("title", "x")
	fw, _ := mw.CreateFormFile("file", "a.txt")
	_, _ = fw.Write([]byte("data"))
	_ = mw.Close()
	require.Equal(t, []string{"title", "file"}, bodyKeys(mw.FormDataContentType(), buf.Bytes()))
}

func TestComputeMetrics(t *testing.T) {
	hits := []hit{
		{Method: "GET", Path: "/products/1", Status: 200},
		{Method: "GET", Path: "/products/2", Params: []string{"q"}, Status: 200},
		{Method: "POST", Path: "/login", Params: []string{"user", "pass"}, Body: true, Status: 302},
		{Method: "POST", Path: "/login", Params: []string{"pass", "user"}, Body: true, Status: 200},
		{Method: "GET", Path: "/nope", Status: 404},
		{Method: "GET", Path: "/boom", Status: 500},
	}
	m := computeMetrics(hits, 1.5)
	require.Equal(t, Metrics{Routes: 3, Params: 3, Forms: 1, Requests: 6, Errors: 1, Seconds: 1.5}, m)
}

func TestMedian(t *testing.T) {
	m := median([]Metrics{{Routes: 10, Seconds: 3}, {Routes: 50, Seconds: 1}, {Routes: 12, Seconds: 2}})
	require.Equal(t, 12, m.Routes)
	require.Equal(t, 2.0, m.Seconds)
}

func TestDelta(t *testing.T) {
	require.Equal(t, "+50%", delta(10, 15))
	require.Equal(t, "-20%", delta(10, 8))
	require.Equal(t, "=", delta(10, 10))
	require.Equal(t, "new", delta(0, 3))
}
