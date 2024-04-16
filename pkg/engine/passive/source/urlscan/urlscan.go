package urlscan

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	jsoniter "github.com/json-iterator/go"
	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/passive/httpclient"
	"github.com/projectdiscovery/katana/pkg/engine/passive/source"
	urlutil "github.com/projectdiscovery/utils/url"
)

type response struct {
	Results []Result `json:"results"`
	HasMore bool     `json:"has_more"`
}

type Result struct {
	Page Page          `json:"page"`
	Sort []interface{} `json:"sort"`
}

type Page struct {
	Url string `json:"url"`
}

type Source struct {
	apiKeys []string
}

func (s *Source) Run(ctx context.Context, sharedCtx *common.Shared, rootUrl string) <-chan source.Result {
	results := make(chan source.Result)

	go func() {
		defer close(results)

		if parsedRootUrl, err := urlutil.Parse(rootUrl); err == nil {
			rootUrl = parsedRootUrl.Hostname()
		}
		httpClient := httpclient.NewHttpClient(sharedCtx.Options.Options.Timeout)

		randomApiKey := source.PickRandom(s.apiKeys, s.Name())
		if randomApiKey == "" {
			return
		}

		var searchAfter string
		hasMore := true
		headers := map[string]string{"API-Key": randomApiKey}
		apiURL := fmt.Sprintf("https://urlscan.io/api/v1/search/?q=domain:%s&size=10000", rootUrl)
		for hasMore {
			if searchAfter != "" {
				apiURL = fmt.Sprintf("%s&search_after=%s", apiURL, searchAfter)
			}

			resp, err := httpClient.Get(ctx, apiURL, "", headers)
			if err != nil {
				results <- source.Result{Source: s.Name(), Error: err}
				httpClient.DiscardHTTPResponse(resp)
				return
			}

			var data response
			err = jsoniter.NewDecoder(resp.Body).Decode(&data)
			if err != nil {
				results <- source.Result{Source: s.Name(), Error: err}
				resp.Body.Close()
				return
			}
			resp.Body.Close()

			if resp.StatusCode == http.StatusTooManyRequests {
				results <- source.Result{Source: s.Name(), Error: fmt.Errorf("urlscan rate limited")}
				return
			}

			for _, url := range data.Results {
				results <- source.Result{Source: s.Name(), Value: url.Page.Url, Reference: apiURL}
			}
			if len(data.Results) > 0 {
				lastResult := data.Results[len(data.Results)-1]
				if len(lastResult.Sort) > 0 {
					sort1 := strconv.Itoa(int(lastResult.Sort[0].(float64)))
					sort2, _ := lastResult.Sort[1].(string)

					searchAfter = fmt.Sprintf("%s,%s", sort1, sort2)
				}
			}
			hasMore = data.HasMore
		}
	}()

	return results
}

func (s *Source) Name() string {
	return "urlscan"
}

func (s *Source) NeedsKey() bool {
	return true
}

func (s *Source) AddApiKeys(keys []string) {
	s.apiKeys = keys
}
