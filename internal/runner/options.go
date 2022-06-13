package runner

import (
	"bufio"
	"errors"
	"os"
	"strings"

	"github.com/projectdiscovery/katana/pkg/types"
)

// validateOptions validates the provided options for crawler
func validateOptions(options *types.Options) error {
	if options.MaxDepth <= 0 && options.CrawlDuration <= 0 {
		return errors.New("either max-depth or crawl-duration must be specified")
	}
	return nil
}

// hasStdin returns true if we have stdin input
func hasStdin() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	if fi.Mode()&os.ModeNamedPipe == 0 {
		return false
	}
	return true
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
