package export

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/stretchr/testify/require"
)

const testEndpoint = "http://127.0.0.1:8080/traffic"

func TestWrapFromEnvironment(t *testing.T) {
	inner := &recordingWriter{}

	t.Run("disabled returns inner", func(t *testing.T) {
		t.Setenv(EnvEndpoint, "")
		t.Setenv(EnvToken, "")
		w, err := Wrap(inner)
		require.NoError(t, err)
		require.Same(t, inner, w)
	})

	t.Run("partial configuration keeps inner", func(t *testing.T) {
		t.Setenv(EnvEndpoint, "https://collector.example/traffic")
		t.Setenv(EnvToken, "")
		w, err := Wrap(inner)
		require.Error(t, err)
		require.Same(t, inner, w)
	})

	t.Run("invalid attributes keep inner", func(t *testing.T) {
		t.Setenv(EnvEndpoint, "https://collector.example/traffic")
		t.Setenv(EnvToken, "test-token")
		t.Setenv(EnvAttributes, `{"run":1}`)
		w, err := Wrap(inner)
		require.ErrorContains(t, err, EnvAttributes)
		require.Same(t, inner, w)
	})
}

func TestValidateEndpoint(t *testing.T) {
	tests := []struct {
		endpoint      string
		allowInsecure bool
		wantError     bool
	}{
		{endpoint: "https://collector.example/traffic"},
		{endpoint: "http://127.0.0.1:8080/traffic"},
		{endpoint: "http://localhost:8080/traffic"},
		{endpoint: "http://collector.internal/traffic", allowInsecure: true},
		{endpoint: "http://collector.example/traffic", wantError: true},
		{endpoint: "https://user@collector.example/traffic", wantError: true},
		{endpoint: "https://collector.example/traffic?next=elsewhere", wantError: true},
		{endpoint: "https://collector.example/traffic#fragment", wantError: true},
		{endpoint: "file:///traffic", wantError: true},
	}
	for _, test := range tests {
		t.Run(test.endpoint, func(t *testing.T) {
			err := validateEndpoint(test.endpoint, test.allowInsecure)
			if test.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestEncodeAttributes(t *testing.T) {
	_, err := encodeAttributes(map[string]string{"request": "x"})
	require.Error(t, err)
	_, err = encodeAttributes(map[string]string{"Bad-Key": "x"})
	require.Error(t, err)
	_, err = encodeAttributes(map[string]string{"run_id": strings.Repeat("a", maxAttrValue+1)})
	require.ErrorContains(t, err, "exceeds")
	encoded, err := encodeAttributes(nil)
	require.NoError(t, err)
	require.Nil(t, encoded)
}

func TestWriterCaptureAndDeliver(t *testing.T) {
	type delivery struct {
		path, auth string
		events     []map[string]any
	}
	received := make(chan delivery, 1)
	client := &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
		var batch struct {
			BatchID string           `json:"batch_id"`
			Events  []map[string]any `json:"events"`
		}
		if err := json.NewDecoder(r.Body).Decode(&batch); err != nil {
			return nil, err
		}
		require.NotEmpty(t, batch.BatchID)
		received <- delivery{path: r.URL.Path, auth: r.Header.Get("Authorization"), events: batch.Events}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{}`))}, nil
	})}

	inner := &recordingWriter{}
	w, err := New(inner, Config{
		Endpoint:   testEndpoint,
		Token:      "test-token",
		Attributes: map[string]string{"run_id": "run-1"},
		HTTPClient: client,
	})
	require.NoError(t, err)

	httpReq, _ := http.NewRequest(http.MethodPost, "https://target.example/api", nil)
	httpReq.Proto = "HTTP/2.0"
	body := strings.Repeat("a", maxBodyBytes+10)
	result := &output.Result{
		Timestamp: time.Date(2026, time.September, 24, 12, 30, 0, 0, time.UTC),
		Request: &navigation.Request{
			Method:  http.MethodPost,
			URL:     "https://target.example/api",
			Headers: map[string]string{"Content-Type": "application/json"},
			Body:    `{"key":"value"}`,
		},
		Response: &navigation.Response{
			StatusCode: http.StatusCreated,
			Headers:    navigation.Headers{"Content-Type": "application/json"},
			Body:       body,
			Resp: &http.Response{
				StatusCode: http.StatusCreated,
				Proto:      "HTTP/2.0",
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Request:    httpReq,
			},
		},
	}
	capturer := w.(interface{ Capture(*output.Result) })
	capturer.Capture(result)
	// Results without a fetched response are not exported.
	capturer.Capture(&output.Result{Request: &navigation.Request{Method: http.MethodGet, URL: "https://target.example/found"}})
	// Write only delegates to the inner writer.
	require.NoError(t, w.Write(result))
	require.NoError(t, w.Close())

	require.Len(t, inner.results, 1)
	require.True(t, inner.closed)

	got := <-received
	require.Equal(t, "/traffic", got.path)
	require.Equal(t, "Bearer test-token", got.auth)
	require.Len(t, got.events, 1)
	ev := got.events[0]
	require.Equal(t, "run-1", ev["run_id"])
	require.Equal(t, "cli", ev["source_type"])
	require.Equal(t, "katana", ev["process"])
	require.Equal(t, "2026-09-24T12:30:00Z", ev["timestamp"])

	req := ev["request"].(map[string]any)
	require.Equal(t, "POST", req["method"])
	require.Equal(t, "HTTP/2.0", req["http_version"])
	require.Equal(t, `{"key":"value"}`, req["body_preview"])

	resp := ev["response"].(map[string]any)
	require.EqualValues(t, http.StatusCreated, resp["status"])
	require.Equal(t, "Created", resp["status_text"])
	require.Equal(t, "application/json", resp["content_type"])
	require.Len(t, resp["body_preview"], maxBodyBytes)
	require.EqualValues(t, len(body), resp["body_size"])
	require.Equal(t, true, resp["body_truncated"])
	require.Equal(t, sha256Hex(body), resp["body_sha256"])
	require.NotContains(t, resp, "method")
}

func TestWriterSkipsBinaryPreview(t *testing.T) {
	w := &writer{}
	encoded, ok := w.encode(&output.Result{
		Request: &navigation.Request{Method: http.MethodGet, URL: "https://target.example/file.png"},
		Response: &navigation.Response{
			StatusCode: http.StatusOK,
			Body:       "\x89PNG\r\n",
			Resp:       &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"image/png"}}},
		},
	})
	require.True(t, ok)
	var ev event
	require.NoError(t, json.Unmarshal(encoded, &ev))
	require.Empty(t, ev.Response.BodyPreview)
	require.Equal(t, sha256Hex("\x89PNG\r\n"), ev.Response.BodySHA256)
	require.True(t, ev.Response.BodyTruncated)
}

func TestWriterSkipsNonUTF8Preview(t *testing.T) {
	w := &writer{}
	latin1 := "caf\xe9"
	encoded, ok := w.encode(&output.Result{
		Request: &navigation.Request{Method: http.MethodPost, URL: "https://target.example/form", Body: latin1},
		Response: &navigation.Response{
			StatusCode: http.StatusOK,
			Body:       latin1,
			Resp:       &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"text/html; charset=iso-8859-1"}}},
		},
	})
	require.True(t, ok)
	var ev event
	require.NoError(t, json.Unmarshal(encoded, &ev))
	for _, m := range []message{ev.Request, ev.Response.message} {
		require.Empty(t, m.BodyPreview)
		require.True(t, m.BodyTruncated)
		require.Equal(t, sha256Hex(latin1), m.BodySHA256)
		require.EqualValues(t, len(latin1), m.BodySize)
	}
}

func TestNewMessageTruncatesOnRuneBoundary(t *testing.T) {
	body := strings.Repeat("a", maxBodyBytes-1) + "é" + "tail"
	m := newMessage(http.MethodGet, "https://target.example", "HTTP/1.1", nil, body)
	require.True(t, m.BodyTruncated)
	require.Len(t, m.BodyPreview, maxBodyBytes-1)
	require.Equal(t, sha256Hex(body), m.BodySHA256)
}

func TestWriterReportsFailedDelivery(t *testing.T) {
	client := &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
		return &http.Response{StatusCode: http.StatusBadRequest, Body: io.NopCloser(strings.NewReader(""))}, nil
	})}
	w, err := New(&recordingWriter{}, Config{Endpoint: testEndpoint, Token: "test-token", HTTPClient: client})
	require.NoError(t, err)
	w.(interface{ Capture(*output.Result) }).Capture(&output.Result{
		Request:  &navigation.Request{Method: http.MethodGet, URL: "https://target.example"},
		Response: &navigation.Response{StatusCode: http.StatusOK, Resp: &http.Response{StatusCode: http.StatusOK, Header: http.Header{}}},
	})
	require.ErrorContains(t, w.Close(), "status 400")
}

type recordingWriter struct {
	results []*output.Result
	closed  bool
}

func (r *recordingWriter) Write(result *output.Result) error {
	r.results = append(r.results, result)
	return nil
}
func (r *recordingWriter) WriteErr(*output.Error) error { return nil }
func (r *recordingWriter) Close() error                 { r.closed = true; return nil }
func (r *recordingWriter) GetResultCount() int64        { return int64(len(r.results)) }

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
