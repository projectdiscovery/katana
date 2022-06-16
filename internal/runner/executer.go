package runner

import (
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/standard"
)

// ExecuteCrawling executes the crawling main loop
func (r *Runner) ExecuteCrawling() error {
	inputs := r.parseInputs()
	if len(inputs) == 0 {
		return errors.New("no input provided for crawling")
	}

	crawler, err := standard.New(r.crawlerOptions)
	if err != nil {
		return errors.Wrap(err, "could not create standard crawler")
	}
	defer crawler.Close()

	for _, input := range inputs {
		crawler.Crawl(input)
	}
	return nil
}
