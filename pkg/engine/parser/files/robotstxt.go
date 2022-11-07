package files

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/retryablehttp-go"
)

type robotsTxtCrawler struct {
	httpclient *retryablehttp.Client
}

// Visit visits the provided URL with file crawlers
func (r *robotsTxtCrawler) Visit(URL string, callback func(navigation.Request)) error {
	URL = strings.TrimSuffix(URL, "/")
	requestURL := fmt.Sprintf("%s/robots.txt", URL)
	req, err := retryablehttp.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return errors.Wrap(err, "could not create request")
	}
	req.Header.Set("User-Agent", utils.WebUserAgent())

	resp, err := r.httpclient.Do(req)
	if err != nil {
		return errors.Wrap(err, "could not do request")
	}
	defer resp.Body.Close()

	r.parseReader(resp.Body, resp, callback)
	return nil
}

func (r *robotsTxtCrawler) parseReader(reader io.Reader, resp *http.Response, callback func(navigation.Request)) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		text := scanner.Text()
		splitted := strings.SplitN(text, ": ", 2)
		if len(splitted) < 2 {
			continue
		}
		directive := strings.ToLower(splitted[0])
		if strings.HasPrefix(directive, "allow") || strings.EqualFold(directive, "disallow") {
			callback(navigation.NewNavigationRequestURLFromResponse(strings.Trim(splitted[1], " "), resp.Request.URL.String(), "file", "robotstxt", navigation.Response{
				Depth: 2,
				Resp:  resp,
			}))
		}
	}
}
