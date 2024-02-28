package passive

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/passive/httpclient"
	"github.com/projectdiscovery/katana/pkg/engine/passive/source"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	errorutil "github.com/projectdiscovery/utils/errors"
	urlutil "github.com/projectdiscovery/utils/url"
	"golang.org/x/exp/maps"
)

// Crawler is a passive crawler instance
type Crawler struct {
	*common.Shared
	sources    []source.Source
	httpClient *httpclient.HttpClient
}

// New returns a new passive crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	shared, err := common.NewShared(options)
	if err != nil {
		return nil, errorutil.NewWithErr(err).WithTag("passive")
	}

	sources := make(map[string]source.Source, len(Sources))
	if len(options.Options.PassiveSource) > 0 {
		for _, source := range options.Options.PassiveSource {
			if s, ok := Sources[source]; ok {
				sources[source] = s
			}
		}
	} else {
		sources = Sources
	}

	if len(sources) == 0 {
		gologger.Fatal().Msg("No sources selected for this search")
	}

	gologger.Debug().Msgf(fmt.Sprintf("Selected source(s) for this crawl: %s", strings.Join(maps.Keys(sources), ", ")))

	httpClient := httpclient.NewHttpClient(options.Options.Timeout)
	return &Crawler{Shared: shared, sources: maps.Values(sources), httpClient: httpClient}, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	return nil
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(rootURL string) error {
	rootUrlParsed, _ := urlutil.ParseURL(rootURL, true)
	results := make(chan source.Result)
	go func() {
		defer close(results)

		ctx := context.Background()
		wg := &sync.WaitGroup{}
		for _, s := range c.sources {
			wg.Add(1)
			go func(source source.Source) {
				for result := range source.Run(ctx, c.Shared, rootURL) {
					results <- result
				}
				wg.Done()
			}(s)
		}
		wg.Wait()
	}()

	for result := range results {
		if !utils.IsURL(result.Value) {
			gologger.Debug().Msgf("`%v` not a url. skipping", result.Value)
			continue
		}

		if ok, err := c.Options.ValidateScope(result.Value, rootUrlParsed.Hostname()); err != nil || !ok {
			gologger.Debug().Msgf("`%v` not in scope. skipping", result.Value)
			continue
		}

		req := &navigation.Request{Method: "GET", URL: result.Value}
		resp := &navigation.Response{}
		c.Output(req, resp, nil)
	}
	return nil
}
