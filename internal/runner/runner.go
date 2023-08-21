package runner

import (
	"encoding/json"
	"os"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine"
	"github.com/projectdiscovery/katana/pkg/engine/hybrid"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/engine/standard"
	"github.com/projectdiscovery/katana/pkg/types"
	errorutil "github.com/projectdiscovery/utils/errors"
	fileutil "github.com/projectdiscovery/utils/file"
	mapsutil "github.com/projectdiscovery/utils/maps"
	updateutils "github.com/projectdiscovery/utils/update"
	"go.uber.org/multierr"
)

// Runner creates the required resources for crawling
// and executes the crawl process.
type Runner struct {
	crawlerOptions *types.CrawlerOptions
	stdin          bool
	crawler        engine.Engine
	options        *types.Options
	state          *RunnerState
}

type RunnerState struct {
	InFlightUrls *mapsutil.SyncLockMap[string, struct{}]
}

// New returns a new crawl runner structure
func New(options *types.Options) (*Runner, error) {
	// create the resume configuration structure
	if options.ShouldResume() {
		gologger.Info().Msg("Resuming from save checkpoint")

		file, err := os.ReadFile(options.Resume)
		if err != nil {
			return nil, err
		}

		runnerState := &RunnerState{}
		err = json.Unmarshal(file, runnerState)
		if err != nil {
			return nil, err
		}
		options.URLs = mapsutil.GetKeys(runnerState.InFlightUrls.GetAll())
	}

	configureOutput(options)
	showBanner()

	if options.Version {
		gologger.Info().Msgf("Current version: %s", version)
		return nil, nil
	}

	if !options.DisableUpdateCheck {
		latestVersion, err := updateutils.GetToolVersionCallback("katana", version)()
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
	runner := &Runner{options: options, stdin: fileutil.HasStdin(), crawlerOptions: crawlerOptions, crawler: crawler, state: &RunnerState{InFlightUrls: mapsutil.NewSyncLockMap[string, struct{}]()}}

	return runner, nil
}

// Close closes the runner releasing resources
func (r *Runner) Close() error {
	return multierr.Combine(
		r.crawler.Close(),
		r.crawlerOptions.Close(),
	)
}

func (r *Runner) SaveState(resumeFilename string) error {
	runnerState := r.state
	data, _ := json.Marshal(runnerState)
	return os.WriteFile(resumeFilename, data, os.ModePerm)
}
