package passive

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/passive/httpclient"
	"github.com/projectdiscovery/katana/pkg/engine/passive/source"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	errorutil "github.com/projectdiscovery/utils/errors"
	fileutil "github.com/projectdiscovery/utils/file"
	urlutil "github.com/projectdiscovery/utils/url"
	"golang.org/x/exp/maps"
	"gopkg.in/yaml.v2"
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

	// Load the passive providers info from the file
	if options.Options.Passive && fileutil.FileExists(options.Options.PassiveProviderConfig) {
		gologger.Info().Msgf("Loading provider config from %s", options.Options.PassiveProviderConfig)

		if err := loadPassiveProvidersFrom(options.Options.PassiveProviderConfig); err != nil && (!strings.Contains(err.Error(), "file doesn't exist") || errors.Is(os.ErrNotExist, err)) {
			gologger.Error().Msgf("Could not read providers from %s: %s\n", options.Options.PassiveProviderConfig, err)
		}
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
	gologger.Info().Msgf("Enumerating passive endpoints for %s", rootURL)

	rootUrlParsed, _ := urlutil.ParseURL(rootURL, true)
	results := make(chan source.Result)
	var timeTaken time.Duration
	go func() {
		defer func(startTime time.Time) {
			timeTaken = time.Since(startTime)
			close(results)
		}(time.Now())

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

	seenURLs := make(map[string]struct{})
	sourceStats := make(map[string]int)
	for result := range results {
		if _, found := seenURLs[result.Value]; found {
			continue
		}

		if !utils.IsURL(result.Value) {
			gologger.Debug().Msgf("`%v` not a url. skipping", result.Value)
			continue
		}

		if ok, err := c.Options.ValidateScope(result.Value, rootUrlParsed.Hostname()); err != nil || !ok {
			gologger.Debug().Msgf("`%v` not in scope. skipping", result.Value)
			continue
		}

		if !c.Options.ExtensionsValidator.ValidatePath(result.Value) {
			gologger.Debug().Msgf("`%v` not allowed extension. skipping", result.Value)
			continue
		}

		seenURLs[result.Value] = struct{}{}
		sourceStats[result.Source]++

		passiveURL, _ := urlutil.Parse(result.Value)
		req := &navigation.Request{
			Method: http.MethodGet,
			URL:    result.Value,
		}
		resp := &navigation.Response{
			StatusCode:   http.StatusOK,
			RootHostname: passiveURL.Hostname(),
			Resp: &http.Response{
				StatusCode: http.StatusOK,
				Request: &http.Request{
					Method: http.MethodGet,
					URL:    passiveURL.URL,
				},
			},
		}
		passiveReference := &navigation.PassiveReference{
			Source:    result.Source,
			Reference: result.Reference,
		}
		c.Output(req, resp, passiveReference, nil)
	}

	var stats []string
	for source, count := range sourceStats {
		stats = append(stats, fmt.Sprintf("%s: %d", source, count))
	}

	gologger.Info().Msgf("Found %d endpoints for %s in %s (%s)", len(seenURLs), rootURL, timeTaken.String(), strings.Join(stats, ", "))
	return nil
}

// loadPassiveProvidersFrom loads the passive providers from a file
func loadPassiveProvidersFrom(file string) error {
	reader, err := fileutil.SubstituteConfigFromEnvVars(file)
	if err != nil {
		return err
	}

	sourceApiKeysMap := map[string][]string{}
	err = yaml.NewDecoder(reader).Decode(sourceApiKeysMap)
	for _, source := range Sources {
		sourceName := strings.ToLower(source.Name())
		apiKeys := sourceApiKeysMap[sourceName]
		if source.NeedsKey() && apiKeys != nil && len(apiKeys) > 0 {
			gologger.Debug().Msgf("API key(s) found for %s.", sourceName)
			source.AddApiKeys(apiKeys)
		}
	}
	return err
}
