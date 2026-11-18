package hybrid

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/projectdiscovery/katana/pkg/utils/scope"
	"github.com/projectdiscovery/ratelimit"
	"github.com/stretchr/testify/require"
)

type nopOutputWriter struct{}

func (w *nopOutputWriter) Close() error                 { return nil }
func (w *nopOutputWriter) Write(*output.Result) error   { return nil }
func (w *nopOutputWriter) WriteErr(*output.Error) error { return nil }
func (w *nopOutputWriter) GetResultCount() int64        { return 0 }

func newTestHybridCrawler(ctx context.Context, limiter *ratelimit.AutoLimiter) *Crawler {
	scopeManager, _ := scope.NewManager(nil, nil, "", true)
	crawlerOpts := &types.CrawlerOptions{
		Options: &types.Options{
			MaxDepth: 2,
			Strategy: "depth-first",
			Context:  ctx,
		},
		OutputWriter:        &nopOutputWriter{},
		HostRateLimit:       limiter,
		ScopeManager:        scopeManager,
		ExtensionsValidator: extensions.NewValidator(nil, nil, true),
	}
	shared, err := common.NewShared(crawlerOpts)
	if err != nil {
		panic(err)
	}
	return &Crawler{Shared: shared}
}

func TestHybridVisitUsesRequestHost(t *testing.T) {
	t.Run("distinct request hosts do not share seed limiter", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		limiter := ratelimit.NewAutoLimiter(ctx, ratelimit.WithMaxCount(1), ratelimit.WithDuration(500*time.Millisecond))
		defer limiter.Stop()

		c := newTestHybridCrawler(ctx, limiter)
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

		session := &common.CrawlSession{
			Ctx:        ctx,
			CancelFunc: cancel,
			Hostname:   "seed.example.com",
			Queue:      q,
		}

		var count atomic.Int32
		start := time.Now()
		err = c.Do(session, func(_ *common.CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			count.Add(1)
			return &navigation.Response{StatusCode: http.StatusOK}, nil
		})
		elapsed := time.Since(start)

		require.NoError(t, err)
		require.Equal(t, int32(3), count.Load())
		require.Less(t, elapsed, 300*time.Millisecond)
	})

	t.Run("throttle applies to request host rather than seed host", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c := newTestHybridCrawler(ctx, nil)

		q1, err := queue.New("depth-first")
		require.NoError(t, err)
		q1.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://throttled.example.com/api",
			Depth:          0,
			SkipValidation: true,
		}, 0)
		s1 := &common.CrawlSession{Ctx: ctx, CancelFunc: cancel, Hostname: "seed.example.com", Queue: q1}
		err = c.Do(s1, func(_ *common.CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{StatusCode: http.StatusTooManyRequests}, nil
		})
		require.NoError(t, err)

		// Visiting the seed host afterwards must NOT be delayed by throttled.example.com's 429.
		q2, err := queue.New("depth-first")
		require.NoError(t, err)
		q2.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://seed.example.com/ok",
			Depth:          0,
			SkipValidation: true,
		}, 0)
		s2 := &common.CrawlSession{Ctx: ctx, CancelFunc: cancel, Hostname: "seed.example.com", Queue: q2}
		start := time.Now()
		err = c.Do(s2, func(_ *common.CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{StatusCode: http.StatusOK}, nil
		})
		require.NoError(t, err)
		require.Less(t, time.Since(start), 200*time.Millisecond)

		// Visiting the throttled host again must incur backoff even under a clean session.
		q3, err := queue.New("depth-first")
		require.NoError(t, err)
		q3.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://throttled.example.com/retry",
			Depth:          0,
			SkipValidation: true,
		}, 0)
		s3 := &common.CrawlSession{Ctx: ctx, CancelFunc: cancel, Hostname: "seed.example.com", Queue: q3}
		start = time.Now()
		err = c.Do(s3, func(_ *common.CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{StatusCode: http.StatusOK}, nil
		})
		require.NoError(t, err)
		require.GreaterOrEqual(t, time.Since(start), 900*time.Millisecond)
	})

	t.Run("redirected 429 attributes throttle to responding host", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		c := newTestHybridCrawler(ctx, nil)

		q1, err := queue.New("depth-first")
		require.NoError(t, err)
		q1.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://redirector.example.com/start",
			Depth:          0,
			SkipValidation: true,
		}, 0)
		s1 := &common.CrawlSession{Ctx: ctx, CancelFunc: cancel, Hostname: "seed.example.com", Queue: q1}
		err = c.Do(s1, func(_ *common.CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{
				StatusCode: http.StatusTooManyRequests,
				Resp: &http.Response{
					StatusCode: http.StatusTooManyRequests,
					Request:    httptest.NewRequest(http.MethodGet, "https://Target.Example.COM/final", nil),
				},
			}, nil
		})
		require.NoError(t, err)

		// Redirector host should remain fast, while target host should incur backoff.
		q2, err := queue.New("depth-first")
		require.NoError(t, err)
		q2.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://redirector.example.com/next",
			Depth:          0,
			SkipValidation: true,
		}, 0)
		s2 := &common.CrawlSession{Ctx: ctx, CancelFunc: cancel, Hostname: "seed.example.com", Queue: q2}
		start := time.Now()
		err = c.Do(s2, func(_ *common.CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{StatusCode: http.StatusOK}, nil
		})
		require.NoError(t, err)
		require.Less(t, time.Since(start), 200*time.Millisecond)

		q3, err := queue.New("depth-first")
		require.NoError(t, err)
		q3.Push(&navigation.Request{
			Method:         http.MethodGet,
			URL:            "https://target.example.com/retry",
			Depth:          0,
			SkipValidation: true,
		}, 0)
		s3 := &common.CrawlSession{Ctx: ctx, CancelFunc: cancel, Hostname: "seed.example.com", Queue: q3}
		start = time.Now()
		err = c.Do(s3, func(_ *common.CrawlSession, _ *navigation.Request) (*navigation.Response, error) {
			return &navigation.Response{StatusCode: http.StatusOK}, nil
		})
		require.NoError(t, err)
		require.GreaterOrEqual(t, time.Since(start), 900*time.Millisecond)
	})
}
