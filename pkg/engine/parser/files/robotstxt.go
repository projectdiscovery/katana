package files

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/projectdiscovery/utils/errkit"
)

type robotsTxtCrawler struct {
	httpclient *retryablehttp.Client
}

// Visit visits the provided URL with file crawlers
func (r *robotsTxtCrawler) Visit(URL string) ([]*navigation.Request, error) {
	URL = strings.TrimSuffix(URL, "/")
	requestURL := fmt.Sprintf("%s/robots.txt", URL)
	req, err := retryablehttp.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, errkit.Wrap(err, "robotscrawler: could not create request")
	}
	req.Header.Set("User-Agent", utils.WebUserAgent())

	resp, err := r.httpclient.Do(req)
	if err != nil {
		return nil, errkit.Wrap(err, "robotscrawler: could not do request")
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			gologger.Error().Msgf("Error closing response body: %v\n", err)
		}
	}()

	return r.parseReader(resp.Body, resp)
}

func (r *robotsTxtCrawler) parseReader(reader io.Reader, resp *http.Response) (navigationRequests []*navigation.Request, err error) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text()
		splitted := strings.SplitN(text, ": ", 2)
		if len(splitted) < 2 {
			continue
		}
		directive := strings.ToLower(splitted[0])
		if strings.HasPrefix(directive, "allow") || strings.EqualFold(directive, "disallow") {
			navResp := &navigation.Response{
				Depth:      2,
				Resp:       resp,
				StatusCode: resp.StatusCode,
				Headers:    utils.FlattenHeaders(resp.Header),
			}
			navRequest := navigation.NewNavigationRequestURLFromResponse(strings.Trim(splitted[1], " "), resp.Request.URL.String(), "file", "robotstxt", navResp)
			navigationRequests = append(navigationRequests, navRequest)
		}
	}
	return
}
