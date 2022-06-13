package runner

import (
	"time"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"go.uber.org/ratelimit"
)

// Runner creates the required resources for crawling
// and executes the crawl process.
type Runner struct {
	stdin bool

	output    output.Writer
	options   *types.Options
	ratelimit ratelimit.Limiter
}

// New returns a new crawl runner structure
func New(options *types.Options) (*Runner, error) {
	runner := &Runner{options: options, stdin: hasStdin()}

	if err := validateOptions(options); err != nil {
		return nil, errors.Wrap(err, "could not validate options")
	}
	if options.RateLimit > 0 {
		runner.ratelimit = ratelimit.New(options.RateLimit)
	} else if options.RateLimitMinute > 0 {
		runner.ratelimit = ratelimit.New(options.RateLimitMinute, ratelimit.Per(60*time.Second))
	}

	outputWriter, err := output.New(!options.NoColors, options.JSON, options.OutputFile)
	if err != nil {
		return nil, errors.Wrap(err, "could not create output writer")
	}
	runner.output = outputWriter
	return runner, nil
}
