package files

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	retryablehttp "github.com/projectdiscovery/retryablehttp-go"
	"github.com/stretchr/testify/require"
)

func TestRequestWithContextCancellation(t *testing.T) {
	// Server that delays response indefinitely
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done() // Block until request context cancels
	}))
	defer srv.Close()

	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := retryablehttp.NewWithHTTPClient(httpClient, retryablehttp.DefaultOptionsSpraying)
	kf := New(client, "robotstxt")

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := kf.RequestWithContext(ctx, srv.URL)
		done <- err
	}()

	// Cancel immediately
	cancel()

	select {
	case err := <-done:
		require.Error(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("RequestWithContext did not return after context cancellation")
	}
}

func TestRequestWithContextNil(t *testing.T) {
	// nil context should not panic (falls back to Background)
	httpClient := &http.Client{Timeout: 5 * time.Second}
	client := retryablehttp.NewWithHTTPClient(httpClient, retryablehttp.DefaultOptionsSpraying)
	kf := New(client, "robotstxt")

	// This will fail because there's no server, but it should not panic
	_, err := kf.RequestWithContext(nil, "http://localhost:1") //nolint
	require.Error(t, err)
}
