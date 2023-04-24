package common

import (
	"fmt"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/retryablehttp-go"
	"github.com/remeh/sizedwaitgroup"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

type PathFuzz struct {
	httpclient *retryablehttp.Client
	Options    *types.CrawlerOptions
}

func NewFuzz(httpclient *retryablehttp.Client, option *types.CrawlerOptions) *PathFuzz {
	return &PathFuzz{
		httpclient: httpclient,
		Options:    option,
	}
}

func (f *PathFuzz) DoFuzz(URL string) (navigationRequests []*navigation.Request, err error) {
	wg := sizedwaitgroup.New(f.Options.Options.Concurrency)
	mutex := sync.Mutex{}
	parsed, err := url.Parse(URL)
	if err != nil {
		return
	}
	hostname := parsed.Hostname()
	for _, path := range f.Options.Options.PathFuzzDict {
		wg.Add()
		go func(path string) {
			defer wg.Done()
			f.Options.RateLimit.Take()
			path = strings.TrimPrefix(path, "/")
			path = strings.TrimSuffix(path, "\n")
			requestURL := fmt.Sprintf("%s://%s/%s", parsed.Scheme, parsed.Host, path)
			req, err := retryablehttp.NewRequest(http.MethodGet, requestURL, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", utils.WebUserAgent())
			resp, err := f.httpclient.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				limitReader := io.LimitReader(resp.Body, int64(f.Options.Options.BodyReadSize))
				data, err := io.ReadAll(limitReader)
				if err != nil {
					return
				}
				if len(string(data)) > 1000 || !strings.Contains(string(data), "404") {
					navReq := &navigation.Request{
						Method:       req.Method,
						URL:          req.URL.String(),
						Depth:        1,
						Source:       URL,
						RootHostname: parsed.Hostname(),
						Tag:          "path-fuzz",
						Headers:      GetHeader(req.Header),
					}
					mutex.Lock()
					navigationRequests = append(navigationRequests, navReq)
					mutex.Unlock()
				}
			} else if resp.StatusCode == 301 {
				locations := resp.Header["Location"]
				if len(locations) == 0 {
					return
				}
				location := locations[0]
				if !utils.IsURL(location) {
					return
				}
				if ok, err := f.Options.ValidateScope(location, hostname); err != nil || !ok {
					return
				}
				if !f.Options.ValidatePath(location) {
					return
				}
				navReq := &navigation.Request{
					Method:       req.Method,
					URL:          location,
					RootHostname: parsed.Hostname(),
					Depth:        2,
					Source:       URL,
				}
				mutex.Lock()
				navigationRequests = append(navigationRequests, navReq)
				mutex.Unlock()

			}
		}(path)
	}
	wg.Wait()
	return
}

func GetHeader(headers http.Header) map[string]string {
	reqHeader := make(map[string]string)
	for k, v := range headers {
		reqHeader[k] = v[0]
	}
	return reqHeader
}
