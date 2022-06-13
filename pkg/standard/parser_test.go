package standard

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/stretchr/testify/require"
)

func TestHeaderParsers(t *testing.T) {
	parsed, _ := url.Parse("https://security-crawl-maze.app/headers/xyz/")

	t.Run("content-location", func(t *testing.T) {
		var gotURL string
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Content-Location": []string{"/test/headers/content-location.found"}}}}
		headerContentLocationParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/headers/content-location.found", gotURL, "could not get correct url")
	})
	t.Run("link", func(t *testing.T) {
		var gotURL string
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Link": []string{"</test/headers/link.found>; rel=\"preload\""}}}}
		headerLinkParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/headers/link.found", gotURL, "could not get correct url")
	})
	t.Run("location", func(t *testing.T) {
		var gotURL string
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Location": []string{"http://security-crawl-maze.app/test/headers/location.found"}}}}
		headerLocationParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "http://security-crawl-maze.app/test/headers/location.found", gotURL, "could not get correct url")
	})
	t.Run("refresh", func(t *testing.T) {
		var gotURL string
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Refresh": []string{"999; url=/test/headers/refresh.found"}}}}
		headerRefreshParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/headers/refresh.found", gotURL, "could not get correct url")
	})
}

func TestBodyParsers(t *testing.T) {
	parsed, _ := url.Parse("https://security-crawl-maze.app/html/body/xyz/")

	t.Run("a", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<a href=/test/html/body/a/href.found>"))
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyATagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/a/href.found", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader("<a ping=/test/html/body/a/ping.found>"))
		resp = navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyATagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/a/ping.found", gotURL, "could not get correct url")
	})
	t.Run("embed", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<embed src=\"/test/html/body/embed/src.found\"></embed>"))
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyEmbedTagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/embed/src.found", gotURL, "could not get correct url")
	})
	t.Run("frame", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<frameset><frame src=\"/test/html/body/frameset/frame/src.found\"></frame></frameset>"))
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyFrameTagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/frameset/frame/src.found", gotURL, "could not get correct url")
	})
	t.Run("iframe", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<iframe src=\"/test/html/body/iframe/src.found\"></iframe>"))
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyIframeTagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/iframe/src.found", gotURL, "could not get correct url")
	})
	t.Run("input", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<input type=\"image\" src=\"/test/html/body/input/src.found\" name=\"test\" value=\"test\">"))
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyInputSrcTagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/input/src.found", gotURL, "could not get correct url")
	})
	t.Run("isindex", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<isindex action=\"/test/html/body/isindex/action.found\"></isindex>"))
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyIsindexActionTagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/isindex/action.found", gotURL, "could not get correct url")
	})
	t.Run("script", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<script src=\"/test/html/body/script/src.found\"></script>"))
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyScriptSrcTagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/script/src.found", gotURL, "could not get correct url")
	})
	t.Run("button", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<form id=\"test\"><button form=\"test\" formaction=\"/test/html/body/form/button/formaction.found\" type=\"submit\">CLICKME</button></form>"))
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyButtonFormactionTagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/form/button/formaction.found", gotURL, "could not get correct url")
	})
	t.Run("form", func(t *testing.T) {
		t.Run("get", func(t *testing.T) {
			var gotURL string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<form action=\"/test/html/body/form/action-get.found\" method=\"GET\"><input type=\"text\" name=\"test1\" value=\"test\"><input type=\"text\" name=\"test2\" value=\"test\"></form>"))
			resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyFormTagParser(resp, func(resp navigationRequest) {
				gotURL = resp.URL
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/form/action-get.found?test1=test&test2=test", gotURL, "could not get correct url")
		})
		t.Run("post", func(t *testing.T) {
			var gotURL string
			var method string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<form action=\"/test/html/body/form/action-post.found\" method=\"POST\" enctype=\"multipart/form-data\"><input type=\"text\" name=\"test1\" value=\"test\"><input type=\"text\" name=\"test2\" value=\"test\"></form>"))
			resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyFormTagParser(resp, func(resp navigationRequest) {
				gotURL = resp.URL
				method = resp.Method
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/form/action-post.found", gotURL, "could not get correct url")
			require.Equal(t, "POST", method, "could not get correct method")
		})
	})

	t.Run("meta-refresh", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<meta http-equiv=\"refresh\" content=\"10; url=/test/html/head/meta/content-redirect.found\">"))
		resp := navigationResponse{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyMetaContentTagParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/head/meta/content-redirect.found", gotURL, "could not get correct url")
	})
}

func TestScriptParsers(t *testing.T) {
	parsed, _ := url.Parse("https://security-crawl-maze.app/html/script/xyz/")

	t.Run("content", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<script>var endpoint='/test/html/script/content.do';</script>"))
		resp := navigationResponse{scrapeJSResponses: true, Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		scriptContentRegexParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/script/content.do", gotURL, "could not get correct url")
	})

	t.Run("js", func(t *testing.T) {
		parsed, _ = url.Parse("https://security-crawl-maze.app/html/script/xyz/data.js")
		var gotURL string
		resp := navigationResponse{scrapeJSResponses: true, Resp: &http.Response{Request: &http.Request{URL: parsed}}, Body: []byte("var endpoint='/test/html/script/body.do';")}
		scriptJSFileRegexParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/script/body.do", gotURL, "could not get correct url")

		parsed, _ = url.Parse("https://security-crawl-maze.app/html/script/xyz/")
		gotURL = ""
		resp = navigationResponse{scrapeJSResponses: true, Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Content-Type": []string{"application/javascript"}}}, Body: []byte("var endpoint='/test/html/script/body-content-type.do';")}
		scriptJSFileRegexParser(resp, func(resp navigationRequest) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/script/body-content-type.do", gotURL, "could not get correct url")

	})
}
