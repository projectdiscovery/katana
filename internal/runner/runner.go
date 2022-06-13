package runner

import (
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/types"
	"go.uber.org/ratelimit"
)

// Runner creates the required resources for crawling
// and executes the crawl process.
type Runner struct {
	crawlerOptions *types.CrawlerOptions
	stdin          bool

	options   *types.Options
	ratelimit ratelimit.Limiter
}

// New returns a new crawl runner structure
func New(options *types.Options) (*Runner, error) {
	runner := &Runner{options: options, stdin: hasStdin()}

	if err := validateOptions(options); err != nil {
		return nil, errors.Wrap(err, "could not validate options")
	}

	crawlerOptions, err := types.NewCrawlerOptions(options)
	if err != nil {
		return nil, errors.Wrap(err, "could not create crawler options")
	}
	runner.crawlerOptions = crawlerOptions
	return runner, nil
}

// Close closes the runner releasing resources
func (r *Runner) Close() error {
	return r.crawlerOptions.Close()
}
