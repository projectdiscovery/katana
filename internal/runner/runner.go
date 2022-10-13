package runner

import (
	"github.com/pkg/errors"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/types"
)

// Runner creates the required resources for crawling
// and executes the crawl process.
type Runner struct {
	crawlerOptions *types.CrawlerOptions
	stdin          bool

	options *types.Options
}

// New returns a new crawl runner structure
func New(options *types.Options) (*Runner, error) {
	configureOutput(options)
	showBanner()

	if options.Version {
		gologger.Info().Msgf("Current version: %s", version)
		return nil, nil
	}
	runner := &Runner{options: options /*stdin: fileutil.HasStdin()*/}

	if err := initExampleFormFillConfig(); err != nil {
		return nil, errors.Wrap(err, "could not init default config")
	}
	if err := validateOptions(options); err != nil {
		return nil, errors.Wrap(err, "could not validate options")
	}
	if options.FormConfig != "" {
		if err := readCustomFormConfig(options); err != nil {
			return nil, err
		}
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
