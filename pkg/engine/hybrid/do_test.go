package hybrid

import (
	"bytes"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

// TestDoKeepsConsumingAfterSlowPage checks that links found by a page that
// loads for longer than the queue idle timeout are still crawled. The queue
// closes its channel after Options.Timeout seconds without items, which used to
// end the crawl while the slow page was still in flight.
func TestDoKeepsConsumingAfterSlowPage(t *testing.T) {
	for _, agents := range []int{1, 3} {
		t.Run(map[int]string{1: "single", 3: "agents"}[agents], func(t *testing.T) {
			opts := hookTestOptions(nil)
			opts.Timeout = 1
			opts.MaxDepth = 3
			options, err := types.NewCrawlerOptions(opts)
			require.NoError(t, err)
			t.Cleanup(func() { _ = options.Close() })
			shared, err := common.NewShared(options)
			require.NoError(t, err)

			c := &Crawler{Shared: shared}
			for i := 0; i < agents; i++ {
				c.agents = append(c.agents, &browserAgent{browser: &rod.Browser{}})
			}

			session, err := c.NewCrawlSessionWithURL("http://example.test/")
			require.NoError(t, err)
			defer session.CancelFunc()

			var mu sync.Mutex
			var visited []string
			pages := map[string]string{
				"/":     `<a href="/slow">slow</a>`,
				"/slow": `<a href="/next">next</a>`,
				"/next": ``,
			}
			doRequest := func(s *common.CrawlSession, req *navigation.Request) (*navigation.Response, error) {
				path := strings.TrimPrefix(req.URL, "http://example.test")
				mu.Lock()
				visited = append(visited, path)
				mu.Unlock()
				if path == "/slow" {
					time.Sleep(2500 * time.Millisecond)
				}
				body := pages[path]
				reader, _ := goquery.NewDocumentFromReader(bytes.NewReader([]byte(body)))
				httpReq, _ := http.NewRequest(http.MethodGet, req.URL, nil)
				return &navigation.Response{
					Resp:         &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": {"text/html"}}, Request: httpReq},
					Body:         body,
					Reader:       reader,
					StatusCode:   http.StatusOK,
					Depth:        req.Depth + 1,
					RootHostname: s.Hostname,
				}, nil
			}

			require.NoError(t, c.Do(session, doRequest))
			mu.Lock()
			defer mu.Unlock()
			require.Contains(t, visited, "/next", "link found by the slow page must be crawled; visited %v", visited)
		})
	}
}
