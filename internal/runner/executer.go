package runner

import (
	"io"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine"
)

// ExecuteCrawling executes the crawling main loop
func (r *Runner) ExecuteCrawling() error {
	inputs := r.parseInputs()
	if len(inputs) == 0 {
		return errors.New("no input provided for crawling")
	}

	crawler, err := engine.New(r.crawlerOptions)
	if err != nil {
		return errors.Wrap(err, "could not create standard crawler")
	}
	defer crawler.Close()

	for _, input := range inputs {
		if err := crawler.Crawl(input); err != nil && !errors.Is(err, io.EOF) {
			gologger.Error().Msgf("Couldn't crawl '%s': %s\n", input, err)
		}
	}
	return nil
}
