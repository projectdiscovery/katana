package alienvault

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/passive/httpclient"
	"github.com/projectdiscovery/katana/pkg/engine/passive/source"
	urlutil "github.com/projectdiscovery/utils/url"
)

type alienvaultResponse struct {
	URLList []url `json:"url_list"`
	HasNext bool  `json:"has_next"`
}

type url struct {
	URL string `json:"url"`
}

type Source struct {
}

func (s *Source) Run(ctx context.Context, sharedCtx *common.Shared, rootUrl string) <-chan source.Result {
	results := make(chan source.Result)

	go func() {
		defer close(results)

		if parsedRootUrl, err := urlutil.Parse(rootUrl); err == nil {
			rootUrl = parsedRootUrl.Hostname()
		}

		httpClient := httpclient.NewHttpClient(sharedCtx.Options.Options.Timeout)
		page := 1
		for {
			apiURL := fmt.Sprintf("https://otx.alienvault.com/api/v1/indicators/domain/%s/url_list?page=%d", rootUrl, page)
			resp, err := httpClient.SimpleGet(ctx, apiURL)
			if err != nil && resp == nil {
				results <- source.Result{Source: s.Name(), Error: err}
				httpClient.DiscardHTTPResponse(resp)
				return
			}

			var response alienvaultResponse
			// Get the response body and decode
			err = json.NewDecoder(resp.Body).Decode(&response)
			if err != nil {
				results <- source.Result{Source: s.Name(), Error: err}
				resp.Body.Close()
				return
			}
			resp.Body.Close()

			for _, record := range response.URLList {
				results <- source.Result{Source: s.Name(), Value: record.URL, Reference: apiURL}
			}

			if !response.HasNext {
				break
			}
			page++
		}
	}()

	return results
}

func (s *Source) Name() string {
	return "alienvault"
}

func (s *Source) NeedsKey() bool {
	return false
}

func (s *Source) AddApiKeys(_ []string) {
	// no key needed
}
