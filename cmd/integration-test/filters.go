package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync/atomic"

	"github.com/projectdiscovery/katana/internal/runner"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
)

var filtersTestcases = map[string]TestCase{
	"match condition":  &matchConditionIntegrationTest{},
	"filter condition": &filterConditionIntegrationTest{},
	"unique filter":    &uniqueFilterIntegrationTest{},
}

type matchConditionIntegrationTest struct{}

// Execute executes a test case and returns an error if occurred
// Execute the docs at ../README.md if the code stops working for integration.
func (h *matchConditionIntegrationTest) Execute() error {
	results, _ := RunKatanaAndGetResults(false,
		"-u", "scanme.sh",
		"-match-condition", "status_code == 200 && contains(body, 'ok')",
	)

	if len(results) != 1 {
		return fmt.Errorf("match condition failed")
	}
	return nil
}

type filterConditionIntegrationTest struct{}

// Execute executes a test case and returns an error if occurred
func (h *filterConditionIntegrationTest) Execute() error {
	results, _ := RunKatanaAndGetResults(false,
		"-u", "scanme.sh",
		"-filter-condition", "status_code == 200 && contains(body, 'ok')",
	)

	if len(results) != 0 {
		return fmt.Errorf("filter condition failed")
	}
	return nil
}

type uniqueFilterIntegrationTest struct{}

func (h *uniqueFilterIntegrationTest) Execute() error {
	// Create a test server that returns 404 for all paths except root
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			w.WriteHeader(http.StatusOK)
			_, _ = fmt.Fprint(w, `<html><body>
				<a href="/page1">Page 1</a>
				<a href="/page2">Page 2</a>
				<a href="/page3">Page 3</a>
				<a href="/page4">Page 4</a>
			</body></html>`)
		} else {
			w.WriteHeader(http.StatusNotFound)
			// Return identical 404 content for all missing pages
			_, _ = fmt.Fprint(w, `<html><body><h1>404 - Page Not Found</h1></body></html>`)
		}
	}))
	defer server.Close()

	options := types.DefaultOptions
	options.URLs = []string{server.URL}
	options.MaxDepth = 2
	options.Concurrency = 1
	options.DisableUniqueFilter = true

	var fourOhFourCount atomic.Int32
	options.OnResult = func(result output.Result) {
		if result.Response.StatusCode == http.StatusNotFound {
			fourOhFourCount.Add(1)
		}
	}

	katanaRunner, err := runner.New(&options)
	if err != nil {
		return fmt.Errorf("could not create runner: %v", err)
	}
	defer func() {
		_ = katanaRunner.Close()
	}()

	if err := katanaRunner.ExecuteCrawling(); err != nil {
		return fmt.Errorf("could not execute crawling: %v", err)
	}

	if fourOhFourCount.Load() != 4 {
		return fmt.Errorf("expected 4 404 responses, got %d", fourOhFourCount.Load())
	}

	return nil
}

// ExtraArgs
var ExtraDebugArgs = []string{}

func RunKatanaAndGetResults(debug bool, extra ...string) ([]string, error) {
	cmd := exec.Command("./katana")
	extra = append(extra, ExtraDebugArgs...)
	cmd.Args = append(cmd.Args, extra...)
	cmd.Args = append(cmd.Args, "-duc") // disable auto updates
	if debug {
		cmd.Args = append(cmd.Args, "-debug")
		cmd.Stderr = os.Stderr
		fmt.Println(cmd.String())
	} else {
		cmd.Args = append(cmd.Args, "-silent")
	}
	data, err := cmd.Output()
	if debug {
		fmt.Println(string(data))
	}
	if len(data) < 1 && err != nil {
		return nil, fmt.Errorf("%v: %v", err.Error(), string(data))
	}
	var parts []string
	items := strings.Split(string(data), "\n")
	for _, i := range items {
		if i != "" {
			parts = append(parts, i)
		}
	}
	return parts, nil
}
