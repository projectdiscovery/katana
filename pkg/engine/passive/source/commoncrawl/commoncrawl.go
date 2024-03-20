// Package commoncrawl logic
package commoncrawl

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/passive/httpclient"
	"github.com/projectdiscovery/katana/pkg/engine/passive/regexp"
	"github.com/projectdiscovery/katana/pkg/engine/passive/source"
)

const (
	indexURL     = "https://index.commoncrawl.org/collinfo.json"
	maxYearsBack = 5
)

var year = time.Now().Year()

type indexResponse struct {
	ID     string `json:"id"`
	APIURL string `json:"cdx-api"`
}

type Source struct {
}

func (s *Source) Run(ctx context.Context, sharedCtx *common.Shared, rootUrl string) <-chan source.Result {
	results := make(chan source.Result)

	go func() {
		defer close(results)

		httpClient := httpclient.NewHttpClient(sharedCtx.Options.Options.Timeout)
		resp, err := httpClient.SimpleGet(ctx, indexURL)
		if err != nil {
			results <- source.Result{Source: s.Name(), Error: err}
			httpClient.DiscardHTTPResponse(resp)
			return
		}

		var indexes []indexResponse
		err = jsoniter.NewDecoder(resp.Body).Decode(&indexes)
		if err != nil {
			results <- source.Result{Source: s.Name(), Error: err}
			resp.Body.Close()
			return
		}
		resp.Body.Close()

		years := make([]string, 0)
		for i := 0; i < maxYearsBack; i++ {
			years = append(years, strconv.Itoa(year-i))
		}

		searchIndexes := make(map[string]string)
		for _, year := range years {
			for _, index := range indexes {
				if strings.Contains(index.ID, year) {
					if _, ok := searchIndexes[year]; !ok {
						searchIndexes[year] = index.APIURL
						break
					}
				}
			}
		}

		for _, apiURL := range searchIndexes {
			further := s.getURLs(ctx, apiURL, rootUrl, httpClient, results)
			if !further {
				break
			}
		}
	}()

	return results
}

func (s *Source) Name() string {
	return "commoncrawl"
}

func (s *Source) NeedsKey() bool {
	return false
}

func (s *Source) AddApiKeys(_ []string) {
	// no key needed
}

func (s *Source) getURLs(ctx context.Context, searchURL, rootURL string, httpClient *httpclient.HttpClient, results chan source.Result) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		default:
			var headers = map[string]string{"Host": "index.commoncrawl.org"}
			currentSearchURL := fmt.Sprintf("%s?url=*.%s", searchURL, rootURL)
			resp, err := httpClient.Get(ctx, currentSearchURL, "", headers)
			if err != nil {
				results <- source.Result{Source: s.Name(), Error: err}
				httpClient.DiscardHTTPResponse(resp)
				return false
			}

			scanner := bufio.NewScanner(resp.Body)

			for scanner.Scan() {
				line := scanner.Text()
				if line == "" {
					continue
				}
				line, _ = url.QueryUnescape(line)
				for _, extractedURL := range regexp.Extract(line) {
					// fix for triple encoded URL
					extractedURL = strings.ToLower(extractedURL)
					extractedURL = strings.TrimPrefix(extractedURL, "25")
					extractedURL = strings.TrimPrefix(extractedURL, "2f")
					if extractedURL != "" {
						results <- source.Result{Source: s.Name(), Value: extractedURL, Reference: currentSearchURL}
					}
				}
			}
			resp.Body.Close()
			return true
		}
	}
}
