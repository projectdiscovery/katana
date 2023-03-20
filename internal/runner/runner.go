package runner

import (
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine"
	"github.com/projectdiscovery/katana/pkg/engine/hybrid"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/engine/standard"
	"github.com/projectdiscovery/katana/pkg/types"
	errorutil "github.com/projectdiscovery/utils/errors"
	fileutil "github.com/projectdiscovery/utils/file"
	"go.uber.org/multierr"
	updateutils "github.com/projectdiscovery/utils/update"
)

// Runner creates the required resources for crawling
// and executes the crawl process.
type Runner struct {
	crawlerOptions *types.CrawlerOptions
	stdin          bool
	crawler        engine.Engine
	options        *types.Options
}

// New returns a new crawl runner structure
func New(options *types.Options) (*Runner, error) {
	configureOutput(options)
	showBanner()

	if options.Version {
		gologger.Info().Msgf("Current version: %s", version)
		return nil, nil
	}

	if !options.DisableUpdateCheck {
		latestVersion, err := updateutils.GetVersionCheckCallback("katana")()
		if err != nil {
			if options.Verbose {
				gologger.Error().Msgf("katana version check failed: %v", err.Error())
			}
		} else {
			gologger.Info().Msgf("Current katana version %v %v", version, updateutils.GetVersionDescription(version, latestVersion))
		}
	}

	if err := initExampleFormFillConfig(); err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not init default config")
	}
	if err := validateOptions(options); err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not validate options")
	}
	if options.FormConfig != "" {
		if err := readCustomFormConfig(options); err != nil {
			return nil, err
		}
	}
	crawlerOptions, err := types.NewCrawlerOptions(options)
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not create crawler options")
	}

	parser.InitWithOptions(options)

	var crawler engine.Engine

	switch {
	case options.Headless:
		crawler, err = hybrid.New(crawlerOptions)
	default:
		crawler, err = standard.New(crawlerOptions)
	}
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not create standard crawler")
	}
	runner := &Runner{options: options, stdin: fileutil.HasStdin(), crawlerOptions: crawlerOptions, crawler: crawler}

	return runner, nil
}

// Close closes the runner releasing resources
func (r *Runner) Close() error {
	return multierr.Combine(
		r.crawler.Close(),
		r.crawlerOptions.Close(),
	)
}
