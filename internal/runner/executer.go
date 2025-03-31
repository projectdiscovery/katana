package runner

import (
	"strings"

	"github.com/projectdiscovery/gologger"
	errorutil "github.com/projectdiscovery/utils/errors"
	urlutil "github.com/projectdiscovery/utils/url"
	"github.com/remeh/sizedwaitgroup"
)

// ExecuteCrawling executes the crawling main loop
func (r *Runner) ExecuteCrawling() error {
	if r.crawler == nil {
		return errorutil.New("crawler is not initialized")
	}
	inputs := r.parseInputs()
	if len(inputs) == 0 {
		return errorutil.New("no input provided for crawling")
	}
	for _, input := range inputs {
		_ = r.state.InFlightUrls.Set(addSchemeIfNotExists(input), struct{}{})
	}

	defer func() {
		if err := r.crawler.Close(); err != nil {
			gologger.Error().Msgf("Error closing crawler: %v\n", err)
		}
	}()

	wg := sizedwaitgroup.New(r.options.Parallelism)
	for _, input := range inputs {
		if !r.networkpolicy.Validate(input) {
			gologger.Info().Msgf("Skipping excluded host %s", input)
			continue
		}
		wg.Add()
		input = addSchemeIfNotExists(input)
		go func(input string) {
			defer wg.Done()

			if err := r.crawler.Crawl(input); err != nil {
				gologger.Warning().Msgf("Could not crawl %s: %s", input, err)
			}
			r.state.InFlightUrls.Delete(input)
		}(input)
	}
	wg.Wait()
	return nil
}

// scheme less urls are skipped and are required for headless mode and other purposes
// this method adds scheme if given input does not have any
func addSchemeIfNotExists(inputURL string) string {
	if strings.HasPrefix(inputURL, urlutil.HTTP) || strings.HasPrefix(inputURL, urlutil.HTTPS) {
		return inputURL
	}
	parsed, err := urlutil.Parse(inputURL)
	if err != nil {
		gologger.Warning().Msgf("input %v is not a valid url got %v", inputURL, err)
		return inputURL
	}
	if parsed.Port() != "" && (parsed.Port() == "80" || parsed.Port() == "8080") {
		return urlutil.HTTP + urlutil.SchemeSeparator + inputURL
	} else {
		return urlutil.HTTPS + urlutil.SchemeSeparator + inputURL
	}
}
