package runner

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/formatter"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/utils/errkit"
	fileutil "github.com/projectdiscovery/utils/file"
	"gopkg.in/yaml.v3"
)

// validateOptions validates the provided options for crawler
func validateOptions(options *types.Options) error {
	if options.MaxDepth <= 0 && options.CrawlDuration.Seconds() <= 0 {
		return errkit.New("either max-depth or crawl-duration must be specified")
	}
	if len(options.URLs) == 0 && !fileutil.HasStdin() {
		return errkit.New("no inputs specified for crawler")
	}

	// Disabling automatic form fill (-aff) for headless navigation due to incorrect implementation.
	// Form filling should be handled via headless actions within the page context
	if options.HeadlessHybrid && options.AutomaticFormFill {
		options.AutomaticFormFill = false
		gologger.Info().Msgf("Automatic form fill (-aff) has been disabled for headless navigation.")
	}

	// Disallow ambiguous engine selection
	if options.Headless && options.HeadlessHybrid {
		return errkit.New("flags -hl (headless) and -hh (hybrid) are mutually exclusive")
	}
	if (options.HeadlessOptionalArguments != nil || options.HeadlessNoSandbox || options.SystemChromePath != "") &&
		!options.Headless && !options.HeadlessHybrid {
		return errkit.New("headless (-hl) or hybrid (-hh) mode is required if -ho, -nos or -scp are set")
	}
	if (options.HeadlessOptionalArguments != nil || options.HeadlessNoSandbox || options.SystemChromePath != "") && !options.Headless && !options.HeadlessHybrid {
		return errkit.New("headless mode (-hl) is required if -ho, -nos or -scp are set")
	}
	if options.SystemChromePath != "" {
		if !fileutil.FileExists(options.SystemChromePath) {
			return errkit.New("specified system chrome binary does not exist")
		}
	}
	if options.StoreResponseDir != "" && !options.StoreResponse {
		gologger.Debug().Msgf("store response directory specified, enabling \"sr\" flag automatically\n")
		options.StoreResponse = true
	}
	for _, mr := range options.OutputMatchRegex {
		cr, err := regexp.Compile(mr)
		if err != nil {
			return errkit.Wrap(err, "Invalid value for match regex option")
		}
		options.MatchRegex = append(options.MatchRegex, cr)
	}
	for _, fr := range options.OutputFilterRegex {
		cr, err := regexp.Compile(fr)
		if err != nil {
			return errkit.Wrap(err, "Invalid value for filter regex option")
		}
		options.FilterRegex = append(options.FilterRegex, cr)
	}
	if options.KnownFiles != "" && options.MaxDepth < 3 {
		gologger.Info().Msgf("Depth automatically set to 3 to accommodate the `--known-files` option (originally set to %d).", options.MaxDepth)
		options.MaxDepth = 3
	}
	gologger.DefaultLogger.SetFormatter(formatter.NewCLI(options.NoColors))
	return nil
}

// readCustomFormConfig reads custom form fill config
func readCustomFormConfig(formConfig string) error {
	file, err := os.Open(formConfig)
	if err != nil {
		return errkit.Wrap(err, "could not read form config")
	}
	defer func() {
		if err := file.Close(); err != nil {
			gologger.Error().Msgf("Error closing file: %v\n", err)
		}
	}()

	var data utils.FormFillData
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return errkit.Wrap(err, "could not decode form config")
	}
	utils.FormData = data
	return nil
}

// parseInputs parses the inputs returning a slice of URLs
func (r *Runner) parseInputs() []string {
	values := make(map[string]struct{})
	for _, url := range r.options.URLs {
		if url == "" {
			continue
		}
		value := normalizeInput(url)
		if _, ok := values[value]; !ok {
			values[value] = struct{}{}
		}
	}
	if r.stdin {
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			value := normalizeInput(scanner.Text())
			if _, ok := values[value]; !ok {
				values[value] = struct{}{}
			}
		}
	}
	final := make([]string, 0, len(values))
	for k := range values {
		final = append(final, k)
	}
	return final
}

func normalizeInput(value string) string {
	return strings.TrimSpace(value)
}

func initExampleFormFillConfig() error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return errkit.Wrap(err, "could not get home directory")
	}
	defaultConfig := filepath.Join(homedir, ".config", "katana", "form-config.yaml")

	if fileutil.FileExists(defaultConfig) {
		return readCustomFormConfig(defaultConfig)
	}
	if err := os.MkdirAll(filepath.Dir(defaultConfig), 0775); err != nil {
		return err
	}
	exampleConfig, err := os.Create(defaultConfig)
	if err != nil {
		return errkit.Wrap(err, "could not get home directory")
	}
	defer func() {
		if err := exampleConfig.Close(); err != nil {
			gologger.Error().Msgf("Error closing example config: %v\n", err)
		}
	}()

	err = yaml.NewEncoder(exampleConfig).Encode(utils.DefaultFormFillData)
	return err
}
