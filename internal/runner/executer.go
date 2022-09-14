package runner

import (
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/standard"
	"github.com/remeh/sizedwaitgroup"
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

	wg := sizedwaitgroup.New(r.options.Parallelism)
	for _, input := range inputs {
		wg.Add()

		go func(input string) {
			defer wg.Done()

			crawler.Crawl(input)
		}(input)
	}
	wg.Wait()
	return nil
}
