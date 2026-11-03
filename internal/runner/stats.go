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
		fmt.Fprintln(os.Stderr, statsLine(time.Since(startTime), done.Load(), total, endpoints))
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

func statsLine(elapsed time.Duration, done int64, total int, endpoints int64) string {
	var percent int64
	if total > 0 {
		percent = done * 100 / int64(total)
	}
	return fmt.Sprintf("[%s] Inputs: %d/%d (%d%%) | Endpoints: %d", formatDuration(elapsed), done, total, percent, endpoints)
}
