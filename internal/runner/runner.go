package runner

import (
	"encoding/json"
	"os"
	"strconv"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine"
	"github.com/projectdiscovery/katana/pkg/engine/dynamic" // Import the dynamic package
	"github.com/projectdiscovery/katana/pkg/engine/hybrid"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/engine/passive"
	"github.com/projectdiscovery/katana/pkg/engine/standard"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/mapcidr"
	"github.com/projectdiscovery/mapcidr/asn"
	"github.com/projectdiscovery/networkpolicy"
	errorutil "github.com/projectdiscovery/utils/errors"
	fileutil "github.com/projectdiscovery/utils/file"
	iputil "github.com/projectdiscovery/utils/ip"
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
	networkpolicy  *networkpolicy.NetworkPolicy
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
	options.ConfigureOutput()
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
		if err := readCustomFormConfig(options.FormConfig); err != nil {
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
	case options.UseDynamicScope: // Add this case for dynamic scoping
		crawler, err = dynamic.New(crawlerOptions)
	case options.Headless:
		crawler, err = hybrid.New(crawlerOptions)
	case options.Passive:
		crawler, err = passive.New(crawlerOptions)
	default:
		crawler, err = standard.New(crawlerOptions)
	}
	if err != nil {
		return nil, errorutil.NewWithErr(err).Msgf("could not create standard crawler")
	}

	var npOptions networkpolicy.Options

	for _, exclude := range options.Exclude {
		switch {
		case exclude == "cdn":
			//implement cdn check in netoworkpolicy pkg??
			continue
		case exclude == "private-ips":
			npOptions.DenyList = append(npOptions.DenyList, networkpolicy.DefaultIPv4Denylist...)
			npOptions.DenyList = append(npOptions.DenyList, networkpolicy.DefaultIPv4DenylistRanges...)
			npOptions.DenyList = append(npOptions.DenyList, networkpolicy.DefaultIPv6Denylist...)
			npOptions.DenyList = append(npOptions.DenyList, networkpolicy.DefaultIPv6DenylistRanges...)
		case iputil.IsCIDR(exclude):
			npOptions.DenyList = append(npOptions.DenyList, exclude)
		case asn.IsASN(exclude):
			// update this to use networkpolicy pkg once https://github.com/projectdiscovery/networkpolicy/pull/55 is merged
			ips := expandASNInputValue(exclude)
			npOptions.DenyList = append(npOptions.DenyList, ips...)
		case iputil.IsPort(exclude):
			port, _ := strconv.Atoi(exclude)
			npOptions.DenyPortList = append(npOptions.DenyPortList, port)
		default:
			npOptions.DenyList = append(npOptions.DenyList, exclude)
		}
	}

	np, _ := networkpolicy.New(npOptions)
	runner := &Runner{
		options:        options,
		stdin:          fileutil.HasStdin(),
		crawlerOptions: crawlerOptions,
		crawler:        crawler,
		state:          &RunnerState{InFlightUrls: mapsutil.NewSyncLockMap[string, struct{}]()},
		networkpolicy:  np,
	}

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

func expandCIDRInputValue(value string) []string {
	var ips []string
	ipsCh, _ := mapcidr.IPAddressesAsStream(value)
	for ip := range ipsCh {
		ips = append(ips, ip)
	}
	return ips
}

func expandASNInputValue(value string) []string {
	var ips []string
	cidrs, _ := asn.GetCIDRsForASNNum(value)
	for _, cidr := range cidrs {
		ips = append(ips, expandCIDRInputValue(cidr.String())...)
	}
	return ips
}
