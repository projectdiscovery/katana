package common

import (
	"net/url"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/parser/files"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	errorutil "github.com/projectdiscovery/utils/errors"
)

type Shared struct {
	Headers    map[string]string
	KnownFiles *files.KnownFiles
	Options    *types.CrawlerOptions
}

func NewShared(options *types.CrawlerOptions) (*Shared, error) {
	shared := &Shared{
		Headers: options.Options.ParseCustomHeaders(),
		Options: options,
	}
	if options.Options.KnownFiles != "" {
		httpclient, _, err := BuildHttpClient(options.Dialer, options.Options, nil)
		if err != nil {
			return nil, errorutil.New("could not create http client").Wrap(err)
		}
		shared.KnownFiles = files.New(httpclient, options.Options.KnownFiles)
	}
	return shared, nil
}

func (s *Shared) Enqueue(queue *queue.Queue, navigationRequests ...navigation.Request) {
	for _, nr := range navigationRequests {
		if nr.URL == "" || !utils.IsURL(nr.URL) {
			continue
		}

		// Ignore blank URL items and only work on unique items
		if !s.Options.UniqueFilter.UniqueURL(nr.RequestURL()) && len(nr.CustomFields) == 0 {
			continue
		}
		// - URLs stuck in a loop
		if s.Options.UniqueFilter.IsCycle(nr.RequestURL()) {
			continue
		}

		scopeValidated := s.ValidateScope(nr.URL, nr.RootHostname)

		// Do not add to crawl queue if max items are reached
		if nr.Depth >= s.Options.Options.MaxDepth || !scopeValidated {
			continue
		}
		queue.Push(nr, nr.Depth)
	}
}

func (s *Shared) ValidateScope(URL string, root string) bool {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return false
	}
	scopeValidated, err := s.Options.ScopeManager.Validate(parsedURL, root)
	return err == nil && scopeValidated
}

func (s *Shared) Output(navigationRequest navigation.Request, navigationResponse *navigation.Response, err error) {
	var errData string
	if err != nil {
		errData = err.Error()
	}
	// Write the found result to output
	result := &output.Result{
		Timestamp: time.Now(),
		Request:   navigationRequest,
		Response:  navigationResponse,
		Error:     errData,
	}

	_ = s.Options.OutputWriter.Write(result)

	if s.Options.Options.OnResult != nil {
		s.Options.Options.OnResult(*result)
	}
}
