package common

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/projectdiscovery/ratelimit"
)

func BenchmarkRateLimit_MixedLatencyHosts(b *testing.B) {
	latencies := []time.Duration{10 * time.Millisecond, 50 * time.Millisecond, 200 * time.Millisecond}
	servers := make([]*httptest.Server, len(latencies))
	for i, lat := range latencies {
		servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(lat)
			w.WriteHeader(http.StatusOK)
		}))
		defer servers[i].Close()
	}

	b.Run("global_limiter", func(b *testing.B) {
		limiter := ratelimit.New(context.Background(), 150, time.Second)
		defer limiter.Stop()

		var idx atomic.Int64
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i := int(idx.Add(1))
				limiter.Take()
				resp, err := http.Get(servers[i%len(servers)].URL)
				if err == nil {
					_ = resp.Body.Close()
				}
			}
		})
	})

	b.Run("per_host_limiter", func(b *testing.B) {
		limiter := ratelimit.NewAutoLimiter(
			context.Background(),
			ratelimit.WithMaxCount(150),
			ratelimit.WithDuration(time.Second),
		)
		defer limiter.Stop()

		var idx atomic.Int64
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i := int(idx.Add(1))
				srv := servers[i%len(servers)]
				_ = limiter.Take(srv.URL)
				resp, err := http.Get(srv.URL)
				if err == nil {
					_ = resp.Body.Close()
				}
			}
		})
	})
}

func BenchmarkRateLimit_ThrottledHost(b *testing.B) {
	var throttleCount atomic.Int64

	fastServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(10 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer fastServer.Close()

	throttledServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if throttleCount.Add(1)%3 == 0 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer throttledServer.Close()

	servers := []*httptest.Server{fastServer, throttledServer}

	b.Run("global_limiter", func(b *testing.B) {
		throttleCount.Store(0)
		limiter := ratelimit.New(context.Background(), 150, time.Second)
		defer limiter.Stop()

		var completed atomic.Int64
		var idx atomic.Int64
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i := int(idx.Add(1))
				limiter.Take()
				resp, err := http.Get(servers[i%len(servers)].URL)
				if err == nil {
					if resp.StatusCode == http.StatusOK {
						completed.Add(1)
					}
					_ = resp.Body.Close()
				}
			}
		})
		b.ReportMetric(float64(completed.Load()), "successful_reqs")
	})

	b.Run("per_host_limiter_with_backoff", func(b *testing.B) {
		throttleCount.Store(0)
		limiter := ratelimit.NewAutoLimiter(
			context.Background(),
			ratelimit.WithMaxCount(150),
			ratelimit.WithDuration(time.Second),
		)
		defer limiter.Stop()

		backoffCache, _ := lru.New[string, *hostBackoff](100)
		shared := &Shared{hostBackoffs: backoffCache}
		var completed atomic.Int64
		var idx atomic.Int64
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				i := int(idx.Add(1))
				srv := servers[i%len(servers)]
				_ = limiter.Take(srv.URL)
				shared.ApplyBackoff(srv.URL)

				resp, err := http.Get(srv.URL)
				if err == nil {
					if IsThrottled(resp.StatusCode) {
						shared.RecordThrottle(srv.URL, resp.StatusCode)
					} else {
						shared.RecordSuccess(srv.URL)
						if resp.StatusCode == http.StatusOK {
							completed.Add(1)
						}
					}
					_ = resp.Body.Close()
				}
			}
		})
		b.ReportMetric(float64(completed.Load()), "successful_reqs")
	})
}

func BenchmarkRateLimit_MultiHostThroughput(b *testing.B) {
	for _, hostCount := range []int{1, 5, 20} {
		servers := make([]*httptest.Server, hostCount)
		for i := range hostCount {
			servers[i] = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				time.Sleep(20 * time.Millisecond)
				w.WriteHeader(http.StatusOK)
			}))
			defer servers[i].Close()
		}

		b.Run(fmt.Sprintf("global_limiter/hosts=%d", hostCount), func(b *testing.B) {
			limiter := ratelimit.New(context.Background(), 150, time.Second)
			defer limiter.Stop()

			var idx atomic.Int64
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := int(idx.Add(1))
					limiter.Take()
					resp, err := http.Get(servers[i%hostCount].URL)
					if err == nil {
						_ = resp.Body.Close()
					}
				}
			})
		})

		b.Run(fmt.Sprintf("per_host_limiter/hosts=%d", hostCount), func(b *testing.B) {
			limiter := ratelimit.NewAutoLimiter(
				context.Background(),
				ratelimit.WithMaxCount(150),
				ratelimit.WithDuration(time.Second),
			)
			defer limiter.Stop()

			var idx atomic.Int64
			b.ResetTimer()
			b.RunParallel(func(pb *testing.PB) {
				for pb.Next() {
					i := int(idx.Add(1))
					srv := servers[i%hostCount]
					_ = limiter.Take(srv.URL)
					resp, err := http.Get(srv.URL)
					if err == nil {
						_ = resp.Body.Close()
					}
				}
			})
		})
	}
}
