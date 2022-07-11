package headless

import (
	"bytes"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"github.com/go-rod/rod/lib/proto"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/urlutil"
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

	// create a new page, but in order to persist the context we should use the same tab
	page, err := headlessEngine.browser.Page(proto.TargetCreateTarget{URL: request.URL})
	if err != nil {
		return response, err
	}
	defer page.Close()

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
	if networkPair, ok := headlessEngine.networkMap[requestURL.String()]; ok {
		response.Resp = networkPair.Response
	}

	response.Reader, err = goquery.NewDocumentFromReader(bytes.NewReader(data))
	if err != nil {
		return response, errors.Wrap(err, "could not make document from reader")
	}
	return response, nil
}
