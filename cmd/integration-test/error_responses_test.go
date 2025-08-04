package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/projectdiscovery/katana/internal/runner"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
)

func TestDisableUniqueFilter(t *testing.T) {
	// Create a test server that returns 404 for all paths except root
	var requestCount atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			fmt.Fprint(w, `<html><body>
				<a href="/page1">Page 1</a>
				<a href="/page2">Page 2</a>
				<a href="/page3">Page 3</a>
				<a href="/page4">Page 4</a>
			</body></html>`)
		} else {
			w.WriteHeader(http.StatusNotFound)
			// Return identical 404 content for all missing pages
			fmt.Fprint(w, `<html><body><h1>404 - Page Not Found</h1></body></html>`)
		}
	}))
	defer server.Close()

	t.Run("With UniqueFilter Enabled (Default)", func(t *testing.T) {
		var callbackCount atomic.Int32
		var fourOhFourCount atomic.Int32
		
		options := &types.Options{
			URLs:                  []string{server.URL},
			MaxDepth:              2,
			Concurrency:           1,
			DisableUniqueFilter: false, // Default behavior
			OnResult: func(result output.Result) {
				callbackCount.Add(1)
				if result.Response.StatusCode == 404 {
					fourOhFourCount.Add(1)
				}
				t.Logf("OnResult: [%d] %s", result.Response.StatusCode, result.Request.URL)
			},
		}
		
		katanaRunner, err := runner.New(options)
		if err != nil {
			t.Fatal(err)
		}
		defer katanaRunner.Close()
		
		if err := katanaRunner.ExecuteCrawling(); err != nil {
			t.Fatal(err)
		}
		
		// With duplicate filtering, we expect only 1 404 response
		if fourOhFourCount.Load() != 1 {
			t.Errorf("Expected 1 404 response, got %d", fourOhFourCount.Load())
		}
	})

	t.Run("With UniqueFilter Disabled", func(t *testing.T) {
		requestCount.Store(0) // Reset counter
		var callbackCount atomic.Int32
		var fourOhFourCount atomic.Int32
		
		options := &types.Options{
			URLs:                  []string{server.URL},
			MaxDepth:              2,
			Concurrency:           1,
			DisableUniqueFilter: true, // New feature
			OnResult: func(result output.Result) {
				callbackCount.Add(1)
				if result.Response.StatusCode == 404 {
					fourOhFourCount.Add(1)
				}
				t.Logf("OnResult: [%d] %s", result.Response.StatusCode, result.Request.URL)
			},
		}
		
		katanaRunner, err := runner.New(options)
		if err != nil {
			t.Fatal(err)
		}
		defer katanaRunner.Close()
		
		if err := katanaRunner.ExecuteCrawling(); err != nil {
			t.Fatal(err)
		}
		
		// With UniqueFilter disabled, we expect all 4 404 responses
		if fourOhFourCount.Load() != 4 {
			t.Errorf("Expected 4 404 responses, got %d", fourOhFourCount.Load())
		}
	})
}