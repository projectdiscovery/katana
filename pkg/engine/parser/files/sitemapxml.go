package files

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/retryablehttp-go"
	errorutil "github.com/projectdiscovery/utils/errors"
)

type sitemapXmlCrawler struct {
	httpclient *retryablehttp.Client
}

// Visit visits the provided URL with file crawlers
func (r *sitemapXmlCrawler) Visit(URL string, callback func(navigation.Request)) error {
	URL = strings.TrimSuffix(URL, "/")
	requestURL := fmt.Sprintf("%s/sitemap.xml", URL)
	req, err := retryablehttp.NewRequest(http.MethodGet, requestURL, nil)
	if err != nil {
		return errorutil.NewWithTag("sitemapcrawler", "could not create request").Wrap(err)
	}
	req.Header.Set("User-Agent", utils.WebUserAgent())

	resp, err := r.httpclient.Do(req)
	if err != nil {
		return errorutil.NewWithTag("sitemapcrawler", "could not do request").Wrap(err)
	}
	defer resp.Body.Close()

	if err := r.parseReader(resp.Body, resp, callback); err != nil {
		return errorutil.NewWithTag("sitemapcrawler", "could not parse sitemap").Wrap(err)
	}
	return nil
}

type sitemapStruct struct {
	URLs    []parsedURL `xml:"url"`
	Sitemap []parsedURL `xml:"sitemap"`
}

type parsedURL struct {
	Loc string `xml:"loc"`
}

func (r *sitemapXmlCrawler) parseReader(reader io.Reader, resp *http.Response, callback func(navigation.Request)) error {
	sitemap := sitemapStruct{}
	if err := xml.NewDecoder(reader).Decode(&sitemap); err != nil {
		return errorutil.NewWithTag("sitemapcrawler", "could not decode xml").Wrap(err)
	}
	for _, url := range sitemap.URLs {
		callback(navigation.NewNavigationRequestURLFromResponse(strings.Trim(url.Loc, " \t\n"), resp.Request.URL.String(), "file", "sitemapxml", navigation.Response{
			Depth: 2,
			Resp:  resp,
		}))
	}
	for _, url := range sitemap.Sitemap {
		callback(navigation.NewNavigationRequestURLFromResponse(strings.Trim(url.Loc, " \t\n"), resp.Request.URL.String(), "file", "sitemapxml", navigation.Response{
			Depth: 2,
			Resp:  resp,
		}))
	}
	return nil
}
