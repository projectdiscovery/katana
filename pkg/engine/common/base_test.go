package common

import (
	"fmt"
	"net/http"
	"sync"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"github.com/stretchr/testify/require"
)

type mockFilter struct {
	seen map[string]bool
}

func newMockFilter() *mockFilter {
	return &mockFilter{seen: make(map[string]bool)}
}

func (f *mockFilter) Close()                          {}
func (f *mockFilter) UniqueContent(_ []byte) bool     { return true }
func (f *mockFilter) IsCycle(_ string) bool            { return false }
func (f *mockFilter) UniqueURL(url string) bool {
	if f.seen[url] {
		return false
	}
	f.seen[url] = true
	return true
}

type mockWriter struct {
	mu      sync.Mutex
	results []*output.Result
}

func (w *mockWriter) Close() error { return nil }
func (w *mockWriter) Write(r *output.Result) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.results = append(w.results, r)
	return nil
}
func (w *mockWriter) WriteErr(_ *output.Error) error { return nil }

func newTestShared(maxDepth int, opts ...func(*types.Options)) (*Shared, *mockWriter) {
	writer := &mockWriter{}
	scopeManager, _ := scope.NewManager(nil, nil, "", true)

	o := &types.Options{
		MaxDepth: maxDepth,
		Strategy: "depth-first",
	}
	for _, fn := range opts {
		fn(o)
	}

	crawlerOpts := &types.CrawlerOptions{
		Options:             o,
		OutputWriter:        writer,
		UniqueFilter:        newMockFilter(),
		ScopeManager:        scopeManager,
		ExtensionsValidator: extensions.NewValidator(nil, nil, true),
	}

	shared := &Shared{
		Options: crawlerOpts,
	}
	return shared, writer
}

func TestEnqueueMaxDepthOutputsDiscoveredURLs(t *testing.T) {
	t.Run("URLs within max depth are enqueued", func(t *testing.T) {
		shared, writer := newTestShared(2)
		q, _ := queue.New("depth-first", 10)

		shared.Enqueue(q, &navigation.Request{
			Method: http.MethodGet,
			URL:    "https://example.com/page",
			Depth:  1,
		})

		require.Equal(t, 0, len(writer.results), "URL within depth should be enqueued, not output")
		item := <-q.Pop()
		require.NotNil(t, item, "URL within depth should be in the queue")
	})

	t.Run("URLs exceeding max depth are output but not enqueued", func(t *testing.T) {
		shared, writer := newTestShared(1)
		q, _ := queue.New("depth-first", 10)

		shared.Enqueue(q, &navigation.Request{
			Method: http.MethodGet,
			URL:    "https://example.com/deep",
			Depth:  2,
		})

		require.Equal(t, 1, len(writer.results), "URL beyond max depth should be output")
		require.Equal(t, "https://example.com/deep", writer.results[0].Request.URL)
		require.Equal(t, ErrMaxDepthReached.Error(), writer.results[0].Error)
		require.Equal(t, 0, q.Len(), "URL beyond max depth should not be in the queue")
	})

	t.Run("URLs at exact max depth are enqueued normally", func(t *testing.T) {
		shared, writer := newTestShared(2)
		q, _ := queue.New("depth-first", 10)

		shared.Enqueue(q, &navigation.Request{
			Method: http.MethodGet,
			URL:    "https://example.com/page",
			Depth:  2,
		})

		require.Equal(t, 0, len(writer.results), "URL at exact max depth should be enqueued, not output")
		item := <-q.Pop()
		require.NotNil(t, item, "URL at exact max depth should be in the queue")
	})

	t.Run("multiple URLs beyond max depth are all output", func(t *testing.T) {
		shared, writer := newTestShared(1)
		q, _ := queue.New("depth-first", 10)

		shared.Enqueue(q,
			&navigation.Request{Method: http.MethodGet, URL: "https://example.com/a", Depth: 2},
			&navigation.Request{Method: http.MethodGet, URL: "https://example.com/b", Depth: 2},
			&navigation.Request{Method: http.MethodGet, URL: "https://example.com/c", Depth: 3},
		)

		require.Equal(t, 3, len(writer.results), "all URLs beyond max depth should be output")
	})

	t.Run("discovered URLs do not consume uniqueness filter", func(t *testing.T) {
		shared, writer := newTestShared(2)
		q, _ := queue.New("depth-first", 10)

		shared.Enqueue(q, &navigation.Request{
			Method: http.MethodGet,
			URL:    "https://example.com/page",
			Depth:  3,
		})
		require.Equal(t, 1, len(writer.results), "URL beyond depth should be output")

		shared.Enqueue(q, &navigation.Request{
			Method: http.MethodGet,
			URL:    "https://example.com/page",
			Depth:  1,
		})

		item := <-q.Pop()
		require.NotNil(t, item, "same URL at valid depth should still be enqueued after being discovered at max depth")
		req := item.(*navigation.Request)
		require.Equal(t, "https://example.com/page", req.URL)
	})
}

func TestEnqueueDomainPageBudget(t *testing.T) {
	withBudget := func(n int) func(*types.Options) {
		return func(o *types.Options) { o.MaxDomainPages = n }
	}

	t.Run("URLs within budget are enqueued", func(t *testing.T) {
		shared, _ := newTestShared(10, withBudget(5))
		q, _ := queue.New("depth-first", 10)

		shared.Enqueue(q, &navigation.Request{
			Method: http.MethodGet, URL: "https://example.com/a", Depth: 1, RootHostname: "example.com",
		})

		require.Equal(t, 1, q.Len())
	})

	t.Run("URLs beyond budget are dropped", func(t *testing.T) {
		shared, _ := newTestShared(10, withBudget(2))
		q, _ := queue.New("depth-first", 10)

		shared.DomainCounter("example.com").Store(2)

		shared.Enqueue(q, &navigation.Request{
			Method: http.MethodGet, URL: "https://example.com/c", Depth: 1, RootHostname: "example.com",
		})

		require.Equal(t, 0, q.Len())
	})

	t.Run("budget is per domain", func(t *testing.T) {
		shared, _ := newTestShared(10, withBudget(1))
		q, _ := queue.New("depth-first", 10)

		shared.DomainCounter("a.com").Store(1)

		shared.Enqueue(q,
			&navigation.Request{Method: http.MethodGet, URL: "https://a.com/x", Depth: 1, RootHostname: "a.com"},
			&navigation.Request{Method: http.MethodGet, URL: "https://b.com/y", Depth: 1, RootHostname: "b.com"},
		)

		require.Equal(t, 1, q.Len())
		item := <-q.Pop()
		req := item.(*navigation.Request)
		require.Equal(t, "https://b.com/y", req.URL)
	})

	t.Run("zero budget means unlimited", func(t *testing.T) {
		shared, _ := newTestShared(10)
		q, _ := queue.New("depth-first", 10)

		for i := range 100 {
			shared.Enqueue(q, &navigation.Request{
				Method: http.MethodGet, URL: fmt.Sprintf("https://example.com/%d", i), Depth: 1, RootHostname: "example.com",
			})
		}

		require.Equal(t, 100, q.Len())
	})
}
