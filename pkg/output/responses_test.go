package output

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

var (
	resp = http.Response{
		StatusCode: 200,
		Status:     "200 OK",
		Proto:      "HTTP/1.1",
		Header:     http.Header{},
		Body:       io.NopCloser(bytes.NewReader([]byte("test body"))),
		Request: &http.Request{
			Method: http.MethodGet,
			URL: &url.URL{
				Scheme: "https",
				Host:   "projectdiscovery.io",
				Path:   "/",
			},
			Host:   "projectdiscovery.io",
			Proto:  "HTTP/1.1",
			Header: http.Header{},
		},
	}
	out = `https://projectdiscovery.io/


GET / HTTP/1.1
Host: projectdiscovery.io
Test: test


HTTP/1.1 200 OK
Test: test

test body`
)

func TestFormatResponses(t *testing.T) {
	tests := []struct {
		Resp   *http.Response
		Result string
	}{
		{Resp: &resp, Result: out},
	}

	w := StandardWriter{}
	for _, test := range tests {
		test.Resp.Request.Header.Add("test", "test")
		test.Resp.Header.Add("test", "test")
		result, err := w.formatResponse(test.Resp)
		require.Nil(t, err)
		require.Equal(t, test.Result, string(result), "could not equal value")
	}
}
