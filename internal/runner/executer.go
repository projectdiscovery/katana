package runner

import (
	"fmt"
	"strings"
	"time"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/utils/filters"
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

	// Track crawl timing
	startTime := time.Now()

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

	// Show completion message with stats
	r.showCompletionStats(startTime)

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

// showCompletionStats shows the final crawl completion message with timing and stats
func (r *Runner) showCompletionStats(startTime time.Time) {
	// Calculate elapsed time
	elapsed := time.Since(startTime)

	// Get total endpoints discovered
	endpointCount := r.crawlerOptions.OutputWriter.GetResultCount()

	// Format elapsed time in human-readable format
	timeStr := formatDuration(elapsed)

	// Show similarity stats first if enabled
	if r.options.SimilarityDeduplication {
		if similarityFilter, ok := r.crawlerOptions.UniqueFilter.(*filters.SimilarityFilter); ok {
			totalProcessed, uniqueDocuments, similarFiltered := similarityFilter.GetStats()
			if totalProcessed > 0 {
				filterRate := float64(similarFiltered) / float64(totalProcessed) * 100
				gologger.Info().Msgf("Similarity detection: %d processed, %d unique, %d filtered - %.1f%% filter rate",
					totalProcessed, uniqueDocuments, similarFiltered, filterRate)
			}
		}
	}

	// Show clean completion message
	gologger.Info().Msgf("Crawl completed in %s. %d endpoints found.", timeStr, endpointCount)
}

// formatDuration formats a duration in human-readable format
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Nanoseconds()/1e6)
	}

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm %ds", minutes, seconds)
	} else {
		return fmt.Sprintf("%ds", seconds)
	}
}
