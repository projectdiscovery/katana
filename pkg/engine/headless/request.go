package headless

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils"
	"github.com/projectdiscovery/urlutil"
	"go.uber.org/multierr"
)

type NetworkPair struct {
	Request  *http.Request
	Response *http.Response
}

// MakeRequest will perform a headless request
func (headlessEngine *HeadlessEngine) MakeRequest(request navigation.Request) (navigation.Response, error) {
	response := navigation.Response{
		Depth:   request.Depth + 1,
		Options: headlessEngine.options,
	}

	var errs []error

	// create a new page, but in order to persist the context we should use the same tab
	page := headlessEngine.pagepool.Get(func() *rod.Page {
		p, err := headlessEngine.browser.Page(proto.TargetCreateTarget{})
		if err != nil {
			errs = append(errs, err)
			return nil
		}
		return p
	})
	if page == nil {
		return response, multierr.Combine(errs...)
	}
	defer headlessEngine.pagepool.Put(page)

	if err := page.Navigate(request.URL); err != nil {
		return response, err
	}

	// we need to process only the HTML as redirects and refreshes are already handled automatically by the browser
	html, err := page.HTML()
	if err != nil {
		return response, err
	}

	limitReader := io.LimitReader(strings.NewReader(html), int64(headlessEngine.options.Options.BodyReadSize))
	data, err := ioutil.ReadAll(limitReader)
	if err != nil {
		return response, err
	}
	response.Body = data

	requestURL, _ := urlutil.Parse(request.RequestURL())
	requestURL = utils.TrimDefaultPort(requestURL)
	if networkPair, ok := headlessEngine.networkMap[requestURL.String()]; ok {
		response.Resp = networkPair.Response
	}
	response.Reader, err = goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return response, errors.Wrap(err, "could not make document from reader")
	}
	return response, nil
}
