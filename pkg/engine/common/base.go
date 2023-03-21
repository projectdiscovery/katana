package common

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/engine/parser/files"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/projectdiscovery/retryablehttp-go"
	errorutil "github.com/projectdiscovery/utils/errors"
	mapsutil "github.com/projectdiscovery/utils/maps"
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

type CrawlSession struct {
	Ctx        context.Context
	CancelFunc context.CancelFunc
	URL        *url.URL
	Hostname   string
	Queue      *queue.Queue
	HttpClient *retryablehttp.Client
}

func (s *Shared) NewCrawlSessionWithURL(URL string) (*CrawlSession, error) {
	ctx, cancel := context.WithCancel(context.Background())
	if s.Options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(s.Options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	parsed, err := url.Parse(URL)
	if err != nil {
		return nil, errorutil.New("could not parse root URL").Wrap(err)
	}
	hostname := parsed.Hostname()

	queue, err := queue.New(s.Options.Options.Strategy, s.Options.Options.Timeout)
	if err != nil {
		return nil, err
	}
	queue.Push(navigation.Request{Method: http.MethodGet, URL: URL, Depth: 0}, 0)

	if s.KnownFiles != nil {
		navigationRequests, err := s.KnownFiles.Request(URL)
		if err != nil {
			gologger.Warning().Msgf("Could not parse known files for %s: %s\n", URL, err)
		}
		s.Enqueue(queue, navigationRequests...)
	}
	httpclient, _, err := BuildHttpClient(s.Options.Dialer, s.Options.Options, func(resp *http.Response, depth int) {
		body, _ := io.ReadAll(resp.Body)
		reader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
		technologies := s.Options.Wappalyzer.Fingerprint(resp.Header, body)
		navigationResponse := navigation.Response{
			Depth:        depth + 1,
			RootHostname: hostname,
			Resp:         resp,
			Body:         string(body),
			Reader:       reader,
			Technologies: mapsutil.GetKeys(technologies),
			StatusCode:   resp.StatusCode,
			Headers:      utils.FlattenHeaders(resp.Header),
		}
		navigationRequests := parser.ParseResponse(navigationResponse)
		s.Enqueue(queue, navigationRequests...)
	})
	if err != nil {
		return nil, errorutil.New("could not create http client").Wrap(err)
	}
	crawlSession := &CrawlSession{
		Ctx:        ctx,
		CancelFunc: cancel,
		URL:        parsed,
		Hostname:   hostname,
		Queue:      queue,
		HttpClient: httpclient,
	}
	return crawlSession, nil
}
