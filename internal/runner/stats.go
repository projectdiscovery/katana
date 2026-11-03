package runner

import (
	"fmt"
	"os"
	"sync/atomic"
	"time"
)

// startStats prints crawl progress every interval, and once more when the
// returned stop is called so short crawls still report. It writes to stderr
// directly, like httpx, so -silent keeps it.
func (r *Runner) startStats(startTime time.Time, done *atomic.Int64, total int) (stop func()) {
	report := func() {
		endpoints := r.crawlerOptions.OutputWriter.GetResultCount()
		fmt.Fprintln(os.Stderr, statsLine(time.Now(), startTime, done.Load(), total, endpoints))
	}
	ticker := time.NewTicker(time.Duration(r.options.StatsInterval) * time.Second)
	quit := make(chan struct{})
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		for {
			select {
			case <-ticker.C:
				report()
			case <-quit:
				return
			}
		}
	}()
	return func() {
		ticker.Stop()
		close(quit)
		<-stopped
		report()
	}
}

// statsLine matches the nuclei and httpx stats layout, with the wall clock added.
func statsLine(now, startTime time.Time, done int64, total int, endpoints int64) string {
	var percent int64
	if total > 0 {
		percent = done * 100 / int64(total)
	}
	elapsed := int(now.Sub(startTime).Seconds())
	return fmt.Sprintf("[%s] [%d:%02d:%02d] | Inputs: %d/%d (%d%%) | Endpoints: %d",
		now.Format(time.DateTime), elapsed/3600, elapsed/60%60, elapsed%60, done, total, percent, endpoints)
}
