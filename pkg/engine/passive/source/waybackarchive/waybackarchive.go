package waybackarchive

import (
	"bufio"
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/projectdiscovery/katana/pkg/engine/common"
	"github.com/projectdiscovery/katana/pkg/engine/passive/httpclient"
	"github.com/projectdiscovery/katana/pkg/engine/passive/regexp"
	"github.com/projectdiscovery/katana/pkg/engine/passive/source"
)

type Source struct {
}

func (s *Source) Run(ctx context.Context, sharedCtx *common.Shared, rootUrl string) <-chan source.Result {
	results := make(chan source.Result)
	go func() {
		defer close(results)

		httpClient := httpclient.NewHttpClient(sharedCtx.Options.Options.Timeout)
		searchURL := fmt.Sprintf("http://web.archive.org/cdx/search/cdx?url=*.%s/*&output=txt&fl=original&collapse=urlkey", rootUrl)
		resp, err := httpClient.Get(ctx, searchURL, "", nil)
		if err != nil {
			results <- source.Result{Source: s.Name(), Error: err}
			return
		}
		defer resp.Body.Close()

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

				results <- source.Result{Source: s.Name(), Value: extractedURL, Reference: searchURL}
			}

		}
	}()

	return results
}

func (s *Source) Name() string {
	return "waybackarchive"
}

func (s *Source) NeedsKey() bool {
	return false
}

func (s *Source) AddApiKeys(_ []string) {
	// no key needed
}
