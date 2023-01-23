package runner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/gologger/formatter"
	"github.com/projectdiscovery/gologger/levels"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	errorutil "github.com/projectdiscovery/utils/errors"
	fileutil "github.com/projectdiscovery/utils/file"
	logutil "github.com/projectdiscovery/utils/log"
	"gopkg.in/yaml.v3"
)

// validateOptions validates the provided options for crawler
func validateOptions(options *types.Options) error {
	if options.MaxDepth <= 0 && options.CrawlDuration <= 0 {
		return errorutil.New("either max-depth or crawl-duration must be specified")
	}
	if options.Verbose {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelVerbose)
	}
	if len(options.URLs) == 0 && !fileutil.HasStdin() {
		return errorutil.New("no inputs specified for crawler")
	}
	if (options.HeadlessOptionalArguments != nil || options.HeadlessNoSandbox || options.SystemChromePath != "") && !options.Headless {
		return errorutil.New("headless mode (-hl) is required if -ho, -nos or -scp are set")
	}
	if options.SystemChromePath != "" {
		if !fileutil.FileExists(options.SystemChromePath) {
			return errorutil.New("specified system chrome binary does not exist")
		}
	}
	if options.StoreResponseDir != "" && !options.StoreResponse {
		gologger.Debug().Msgf("store response directory specified, enabling \"sr\" flag automatically\n")
		options.StoreResponse = true
	}
	if options.Headless && (options.StoreResponse || options.StoreResponseDir != "") {
		return errorutil.New("store responses feature is not supported in headless mode")
	}
	gologger.DefaultLogger.SetFormatter(formatter.NewCLI(options.NoColors))
	return nil
}

// readCustomFormConfig reads custom form fill config
func readCustomFormConfig(options *types.Options) error {
	file, err := os.Open(options.FormConfig)
	if err != nil {
		return errorutil.NewWithErr(err).Msgf("could not read form config")
	}
	defer file.Close()

	var data utils.FormFillData
	if err := yaml.NewDecoder(file).Decode(&data); err != nil {
		return errorutil.NewWithErr(err).Msgf("could not decode form config")
	}
	utils.FormData = data
	return nil
}

// parseInputs parses the inputs returning a slice of URLs
func (r *Runner) parseInputs() []string {
	values := make(map[string]struct{})
	for _, url := range r.options.URLs {
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

// configureOutput configures the output logging levels to be displayed on the screen
func configureOutput(options *types.Options) {
	if options.Silent {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelSilent)
	}
	if options.Verbose {
		gologger.DefaultLogger.SetMaxLevel(levels.LevelVerbose)
	}

	logutil.DisableDefaultLogger()
}

func initExampleFormFillConfig() error {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return errorutil.NewWithErr(err).Msgf("could not get home directory")
	}
	defaultConfig := filepath.Join(homedir, ".config", "katana", "form-config.yaml")

	if fileutil.FileExists(defaultConfig) {
		return nil
	}
	exampleConfig, err := os.Create(defaultConfig)
	if err != nil {
		return errorutil.NewWithErr(err).Msgf("could not get home directory")
	}
	defer exampleConfig.Close()

	err = yaml.NewEncoder(exampleConfig).Encode(utils.DefaultFormFillData)
	return err
}
