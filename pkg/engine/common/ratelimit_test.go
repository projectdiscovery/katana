package common

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/projectdiscovery/ratelimit"
	"github.com/stretchr/testify/require"
)

func TestDoHostRateLimitUsesRequestHost(t *testing.T) {
	t.Run("distinct request hosts do not share seed limiter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		limiter := ratelimit.NewAutoLimiter(ctx, ratelimit.WithMaxCount(1), ratelimit.WithDuration(500*time.Millisecond))
		defer limiter.Stop()

		shared, _ := newTestShared(2, func(o *types.Options) {
			o.Concurrency = 4
		})
		shared.Options.HostRateLimit = limiter
		shared.hostBackoffs = newTestBackoffCache()

		q, err := queue.New("depth-first")
		require.NoError(t, err)
		for _, u := range []string{
			"https://alpha.example.com/1",
			"https://beta.example.com/1",
			"https://gamma.example.com/1",
		} {
			q.Push(&navigation.Request{
				Method:         http.MethodGet,
				URL:            u,
				Depth:          0,
				SkipValidation: true,
			}, 0)
		}

		session := &CrawlSession{
			Ctx:        ctx,
			CancelFunc: cancel,
			Hostname:   "seed.example.com",
			Queue:      q,
		}

		var count atomic.Int32
		start := time.Now()
		err = shared.Do(session, func(_ *CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			count.Add(1)
			return &navigation.Response{StatusCode: http.StatusOK}, nil
		})
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.Equal(t, int32(3), count.Load())
		require.Less(t, elapsed, 300*time.Millisecond)
	})

	t.Run("same request host enforces per-host limit", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		limiter := ratelimit.NewAutoLimiter(ctx, ratelimit.WithUnlimited())
		require.NoError(t, limiter.Add("same.example.com", ratelimit.WithMaxCount(1), ratelimit.WithDuration(300*time.Millisecond)))
		defer limiter.Stop()

		shared, _ := newTestShared(2, func(o *types.Options) {
			o.Concurrency = 4
		})
		shared.Options.HostRateLimit = limiter
		shared.hostBackoffs = newTestBackoffCache()

		q, err := queue.New("depth-first")
		require.NoError(t, err)
		for _, u := range []string{
			"https://same.example.com/1",
			"https://SAME.Example.COM/2",
		} {
			q.Push(&navigation.Request{
				Method:         http.MethodGet,
				URL:            u,
				Depth:          0,
				SkipValidation: true,
			}, 0)
		}

		session := &CrawlSession{
			Ctx:        ctx,
			CancelFunc: cancel,
			Hostname:   "seed.example.com",
			Queue:      q,
		}

		var count atomic.Int32
		start := time.Now()
		err = shared.Do(session, func(_ *CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			count.Add(1)
			return &navigation.Response{StatusCode: http.StatusOK}, nil
		})
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.Equal(t, int32(2), count.Load())
		require.GreaterOrEqual(t, elapsed, 250*time.Millisecond)
	})

	t.Run("concurrent first requests to unregistered host share one bucket", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		limiter := ratelimit.NewAutoLimiter(ctx, ratelimit.WithMaxCount(1), ratelimit.WithDuration(250*time.Millisecond))
		defer limiter.Stop()

		shared, _ := newTestShared(2, func(o *types.Options) {
			o.Concurrency = 4
		})
		shared.Options.HostRateLimit = limiter
		shared.hostBackoffs = newTestBackoffCache()

		q, err := queue.New("depth-first")
		require.NoError(t, err)
		for _, u := range []string{
			"https://unregistered.example.com/1",
			"https://unregistered.example.com/2",
			"https://unregistered.example.com/3",
		} {
			q.Push(&navigation.Request{
				Method:         http.MethodGet,
				URL:            u,
				Depth:          0,
				SkipValidation: true,
			}, 0)
		}

		session := &CrawlSession{
			Ctx:        ctx,
			CancelFunc: cancel,
			Hostname:   "seed.example.com",
			Queue:      q,
		}

		var count atomic.Int32
		start := time.Now()
		err = shared.Do(session, func(_ *CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			count.Add(1)
			return &navigation.Response{StatusCode: http.StatusOK}, nil
		})
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.Equal(t, int32(3), count.Load())
		require.GreaterOrEqual(t, elapsed, 450*time.Millisecond)
	})
}

func TestDoBackoffUsesRequestHost(t *testing.T) {
	t.Run("throttle increments request host instead of seed host", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		shared, _ := newTestShared(2, func(o *types.Options) {
			o.Concurrency = 1
		})
		shared.hostBackoffs = newTestBackoffCache()

		q, err := queue.New("depth-first")
		require.NoError(t, err)
		q.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://throttled.example.com/api",
			Depth:          0,
			SkipValidation: true,
		}, 0)

		session := &CrawlSession{
			Ctx:        ctx,
			CancelFunc: cancel,
			Hostname:   "seed.example.com",
			Queue:      q,
		}

		err = shared.Do(session, func(_ *CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{StatusCode: http.StatusTooManyRequests}, nil
		})
		require.NoError(t, err)
		require.Equal(t, int32(1), shared.backoffFor("throttled.example.com").consecutive.Load())
		require.Equal(t, int32(0), shared.backoffFor("seed.example.com").consecutive.Load())
	})

	t.Run("seed backoff does not delay or decay for other request hosts", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		shared, _ := newTestShared(2, func(o *types.Options) {
			o.Concurrency = 1
		})
		shared.hostBackoffs = newTestBackoffCache()
		shared.backoffFor("seed.example.com").consecutive.Store(1)
		shared.backoffFor("throttled.example.com").consecutive.Store(1)

		q, err := queue.New("depth-first")
		require.NoError(t, err)
		q.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://clean.example.com/ok",
			Depth:          0,
			SkipValidation: true,
		}, 0)

		session := &CrawlSession{
			Ctx:        ctx,
			CancelFunc: cancel,
			Hostname:   "seed.example.com",
			Queue:      q,
		}

		start := time.Now()
		err = shared.Do(session, func(_ *CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{StatusCode: http.StatusOK}, nil
		})
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.Less(t, elapsed, 200*time.Millisecond)
		require.Equal(t, int32(1), shared.backoffFor("seed.example.com").consecutive.Load())
		require.Equal(t, int32(1), shared.backoffFor("throttled.example.com").consecutive.Load())
	})
}

func TestRequestHost(t *testing.T) {
	tests := []struct {
		name     string
		rawURL   string
		fallback string
		want     string
	}{
		{"lowercase host", "https://sub.Example.COM/path?q=1", "Seed.Example.com", "sub.example.com"},
		{"strips port", "https://Example.COM:8443/path", "seed.example.com", "example.com"},
		{"ipv6 with port", "http://[2001:DB8::1]:8080/a", "seed.example.com", "2001:db8::1"},
		{"empty url uses fallback", "", "Seed.Example.com", "seed.example.com"},
		{"relative path uses fallback", "/relative/path", "Seed.Example.com", "seed.example.com"},
		{"invalid url uses fallback", "://missing-scheme", "Seed.Example.com", "seed.example.com"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, RequestHost(tc.rawURL, tc.fallback))
		})
	}
}

func TestDoBackoffUsesRespondingHostOnRedirect(t *testing.T) {
	t.Run("429 after redirect attributes throttle to responding host", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		shared, _ := newTestShared(2, func(o *types.Options) {
			o.Concurrency = 1
		})
		shared.hostBackoffs = newTestBackoffCache()

		q, err := queue.New("depth-first")
		require.NoError(t, err)
		q.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://redirect.example.com/start",
			Depth:          0,
			SkipValidation: true,
		}, 0)

		session := &CrawlSession{
			Ctx:        ctx,
			CancelFunc: cancel,
			Hostname:   "seed.example.com",
			Queue:      q,
		}

		err = shared.Do(session, func(_ *CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{
				StatusCode: http.StatusTooManyRequests,
				Resp: &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Request:    httptest.NewRequest(http.MethodGet, "https://Seed.Example.COM/final", nil),
				},
			}, nil
		})
		require.NoError(t, err)
		require.Equal(t, int32(1), shared.backoffFor("seed.example.com").consecutive.Load())
		require.Equal(t, int32(0), shared.backoffFor("redirect.example.com").consecutive.Load())
	})

	t.Run("200 after redirect decays responding host rather than initial host", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		shared, _ := newTestShared(2, func(o *types.Options) {
			o.Concurrency = 1
		})
		shared.hostBackoffs = newTestBackoffCache()
		shared.backoffFor("seed.example.com").consecutive.Store(1)
		shared.backoffFor("redirect.example.com").consecutive.Store(1)

		q, err := queue.New("depth-first")
		require.NoError(t, err)
		q.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://redirect.example.com/go",
			Depth:          0,
			SkipValidation: true,
		}, 0)

		session := &CrawlSession{
			Ctx:        ctx,
			CancelFunc: cancel,
			Hostname:   "seed.example.com",
			Queue:      q,
		}

		err = shared.Do(session, func(_ *CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{
				StatusCode: http.StatusOK,
				Resp: &http.Response{
					StatusCode: http.StatusOK,
					Request:    httptest.NewRequest(http.MethodGet, "https://seed.example.com/landed", nil),
				},
			}, nil
		})
		require.NoError(t, err)
		require.Equal(t, int32(0), shared.backoffFor("seed.example.com").consecutive.Load())
		require.Equal(t, int32(1), shared.backoffFor("redirect.example.com").consecutive.Load())
	})
}

func TestBackoffForPreservesExistingEntry(t *testing.T) {
	shared, _ := newTestShared(2)
	shared.hostBackoffs = newTestBackoffCache()

	const workers = 16
	results := make([]*hostBackoff, workers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range workers {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			b := shared.backoffFor("Race.Example.COM")
			b.consecutive.Add(1)
			results[idx] = b
		}(i)
	}
	close(start)
	wg.Wait()

	final := shared.backoffFor("race.example.com")
	require.Equal(t, int32(workers), final.consecutive.Load())
	for _, b := range results {
		require.Same(t, final, b)
	}
}
