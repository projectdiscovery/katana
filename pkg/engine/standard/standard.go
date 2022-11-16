package standard

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"sync/atomic"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/dominikbraun/graph"
	"github.com/dominikbraun/graph/draw"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/engine/parser/files"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/remeh/sizedwaitgroup"
)

// Crawler is a standard crawler instance
type Crawler struct {
	headers    map[string]string
	knownFiles *files.KnownFiles
	options    *types.CrawlerOptions
}

// New returns a new standard crawler instance
func New(options *types.CrawlerOptions) (*Crawler, error) {
	crawler := &Crawler{
		headers: options.Options.ParseCustomHeaders(),
		options: options,
	}
	if options.Options.KnownFiles != "" {
		httpclient, _, err := common.BuildClient(options.Dialer, options.Options, nil)
		if err != nil {
			return nil, errors.Wrap(err, "could not create http client")
		}
		crawler.knownFiles = files.New(httpclient, options.Options.KnownFiles)
	}
	return crawler, nil
}

// Close closes the crawler process
func (c *Crawler) Close() error {
	return nil
}

// Crawl crawls a URL with the specified options
func (c *Crawler) Crawl(rootURL string) error {
	parsed, err := url.Parse(rootURL)
	if err != nil {
		return errors.Wrap(err, "could not parse root URL")
	}
	hostname := parsed.Hostname()

	ctx, cancel := context.WithCancel(context.Background())
	if c.options.Options.CrawlDuration > 0 {
		ctx, cancel = context.WithTimeout(ctx, time.Duration(c.options.Options.CrawlDuration)*time.Second)
	}
	defer cancel()

	queue := queue.New(c.options.Options.Strategy)
	queue.Push(navigation.Request{Method: http.MethodGet, URL: rootURL, Depth: 0}, 0)
	parseResponseCallback := c.makeParseResponseCallback(queue)

	if c.knownFiles != nil {
		if err := c.knownFiles.Request(rootURL, func(nr navigation.Request) {
			parseResponseCallback(nr)
		}); err != nil {
			gologger.Warning().Msgf("Could not parse known files for %s: %s\n", rootURL, err)
		}
	}
	httpclient, _, err := common.BuildClient(c.options.Dialer, c.options.Options, func(resp *http.Response, depth int) {
		body, _ := io.ReadAll(resp.Body)
		reader, _ := goquery.NewDocumentFromReader(bytes.NewReader(body))
		parser.ParseResponse(navigation.Response{Depth: depth + 1, Options: c.options, RootHostname: hostname, Resp: resp, Body: body, Reader: reader}, parseResponseCallback)
	})
	if err != nil {
		return errors.Wrap(err, "could not create http client")
	}

	// graph instance
	graphdb := graph.New(navigation.StateHash, graph.Directed())
	states := make(map[uint64]*navigation.State)

	wg := sizedwaitgroup.New(c.options.Options.Concurrency)
	running := int32(0)
	for {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		// Quit the crawling for zero items or context timeout
		if !(atomic.LoadInt32(&running) > 0) && (queue.Len() == 0) {
			break
		}
		item := queue.Pop()
		req, ok := item.(navigation.Request)
		if !ok {
			continue
		}
		if !utils.IsURL(req.URL) {
			continue
		}
		wg.Add()
		atomic.AddInt32(&running, 1)

		go func() {
			defer wg.Done()
			defer atomic.AddInt32(&running, -1)

			c.options.RateLimit.Take()

			// Delay if the user has asked for it
			if c.options.Options.Delay > 0 {
				time.Sleep(time.Duration(c.options.Options.Delay) * time.Second)
			}
			resp, err := c.makeRequest(ctx, req, hostname, req.Depth, httpclient)
			if err != nil {
				gologger.Warning().Msgf("Could not request seed URL: %s\n", err)
				return
			}
			if resp.Resp == nil || resp.Reader == nil {
				return
			}

			// calculate the page state
			if newState, err := navigation.NewState(req, resp); err != nil {
				gologger.Verbose().Msgf("could not create new state for \"%s\": %s", req.URL, err)
			} else {

				// Check if the current state was already visited previously
				// using near approximate search (TODO: current linear complexity => binary search?)
				var existingState *navigation.State
				for _, state := range states {
					similarity := navigation.Similarity(state, newState)
					if similarity >= 99 {
						existingState = state
						break
					}
				}
				if existingState == nil {
					states[newState.Hash] = newState
					// Color edge
					// Html State => Green
					// Static File => Red
					var color string
					if navigation.ContentTypeIsTextHtml(resp.Resp.Header, resp.Body) {
						color = "green"
					} else {
						color = "red"
					}
					_ = graphdb.AddVertex(*newState, graph.VertexAttribute("color", color))
				} else {
					newState = existingState
				}

				// associate the response with the state
				resp.State = newState

				// if req.State is nil => this is a root vertex => nothing to do
				// otherwise we need to create an edge between the previous state and the current one
				if req.State != nil {
					edgeProperties := []func(*graph.EdgeProperties){
						graph.EdgeAttribute("source", req.Source),
						graph.EdgeAttribute("attribute", req.Attribute),
						graph.EdgeAttribute("tag", req.Tag),
						graph.EdgeAttribute("label", fmt.Sprintf("%s\n%s", req.Tag, req.Attribute)),
					}
					_ = graphdb.AddEdge(navigation.StateHash(*req.State), navigation.StateHash(*resp.State), edgeProperties...)
				}
			}

			parser.ParseResponse(resp, parseResponseCallback)
		}()
	}
	wg.Wait()

	if c.options.Options.OutputGraph != "" {
		outputGraphFile, err := os.Create(c.options.Options.OutputGraph)
		if err != nil {
			return err
		}
		defer outputGraphFile.Close()
		if err := draw.DOT(graphdb, outputGraphFile); err != nil {
			return err
		}
	}

	return nil
}

// makeParseResponseCallback returns a parse response function callback
func (c *Crawler) makeParseResponseCallback(queue *queue.VarietyQueue) func(nr navigation.Request) {
	return func(nr navigation.Request) {
		if nr.URL == "" || !utils.IsURL(nr.URL) {
			return
		}
		parsed, err := url.Parse(nr.URL)
		if err != nil {
			return
		}
		// Ignore blank URL items and only work on unique items
		if !c.options.UniqueFilter.UniqueURL(nr.RequestURL()) {
			return
		}

		// Write the found result to output
		result := &output.Result{
			Timestamp: time.Now(),
			Body:      nr.Body,
			URL:       nr.URL,
			Source:    nr.Source,
			Tag:       nr.Tag,
			Attribute: nr.Attribute,
		}
		if nr.Method != http.MethodGet {
			result.Method = nr.Method
		}
		scopeValidated, err := c.options.ScopeManager.Validate(parsed, nr.RootHostname)
		if err != nil {
			return
		}
		if scopeValidated || c.options.Options.DisplayOutScope {
			_ = c.options.OutputWriter.Write(result)
		}

		// Do not add to crawl queue if max items are reached
		if nr.Depth >= c.options.Options.MaxDepth || !scopeValidated {
			return
		}
		queue.Push(nr, nr.Depth)
	}
}
