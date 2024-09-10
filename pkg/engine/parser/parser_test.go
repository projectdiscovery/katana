package parser

import (
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	urlutil "github.com/projectdiscovery/utils/url"
	"github.com/stretchr/testify/require"
)

func TestHeaderParsers(t *testing.T) {
	parsed, _ := urlutil.Parse("https://security-crawl-maze.app/headers/xyz/")

	t.Run("content-location", func(t *testing.T) {
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}, Header: http.Header{"Content-Location": []string{"/test/headers/content-location.found"}}}}
		navigationRequests := headerContentLocationParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/headers/content-location.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("link", func(t *testing.T) {
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}, Header: http.Header{"Link": []string{"</test/headers/link.found>; rel=\"preload\""}}}}
		navigationRequests := headerLinkParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/headers/link.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("location", func(t *testing.T) {
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}, Header: http.Header{"Location": []string{"http://security-crawl-maze.app/test/headers/location.found"}}}}
		navigationRequests := headerLocationParser(resp)
		require.Equal(t, "http://security-crawl-maze.app/test/headers/location.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("refresh", func(t *testing.T) {
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}, Header: http.Header{"Refresh": []string{"999; url=/test/headers/refresh.found"}}}}
		navigationRequests := headerRefreshParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/headers/refresh.found", navigationRequests[0].URL, "could not get correct url")
	})
}

func TestBodyParsers(t *testing.T) {
	parsed, _ := urlutil.Parse("https://security-crawl-maze.app/html/body/xyz/")

	t.Run("a", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<a href=/test/html/body/a/href.found>"))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyATagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/a/href.found", navigationRequests[0].URL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader("<a ping=/test/html/body/a/ping.found>"))
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests = bodyATagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/a/ping.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("background", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<body background=\"/test/html/body/background.found\"></body>"))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyBackgroundTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/background.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("blockquote", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<blockquote cite="/test/html/body/blockquote/cite.found"></blockquote>`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyBlockquoteCiteTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/blockquote/cite.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("area", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<map name="map">
		<area ping="/test/html/body/map/area/ping.found" shape="rect" coords="0,0,150,150" href="#">
	  </map>`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyMapAreaPingTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/map/area/ping.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("audio", func(t *testing.T) {
		t.Run("src", func(t *testing.T) {
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<audio src=\"/test/html/body/audio/src.found\"></audio>"))
			resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			navigationRequests := bodyAudioTagParser(resp)
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/audio/src.found", navigationRequests[0].URL, "could not get correct url")
		})
		t.Run("source", func(t *testing.T) {
			t.Run("src", func(t *testing.T) {
				documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<audio controls><source src=\"/test/html/body/audio/source/src.found\" type=\"audio/mpeg\"></audio>"))
				resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
				navigationRequests := bodyAudioTagParser(resp)
				require.Equal(t, "https://security-crawl-maze.app/test/html/body/audio/source/src.found", navigationRequests[0].URL, "could not get correct url")
			})
			t.Run("srcset", func(t *testing.T) {
				var gotURL []string
				documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<audio controls>
				<source srcset="/test/html/body/audio/source/srcset1x.found 1x,
								/test/html/body/audio/source/srcset2x.found 2x">
			</audio>`))
				resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
				for _, navigationRequest := range bodyAudioTagParser(resp) {
					gotURL = append(gotURL, navigationRequest.URL)
				}
				require.ElementsMatch(t, []string{
					"https://security-crawl-maze.app/test/html/body/audio/source/srcset1x.found",
					"https://security-crawl-maze.app/test/html/body/audio/source/srcset2x.found",
				}, gotURL, "could not get correct url")
			})
		})
	})
	t.Run("img", func(t *testing.T) {
		t.Run("dynsrc", func(t *testing.T) {
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img dynsrc="/test/html/body/img/dynsrc.found">`))
			resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			navigationRequests := bodyImgTagParser(resp)
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/img/dynsrc.found", navigationRequests[0].URL, "could not get correct url")
		})
		t.Run("longdesc", func(t *testing.T) {
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img alt="" src="#" longdesc="/test/html/body/img/longdesc.found">`))
			resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			navigationRequests := bodyImgTagParser(resp)
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/img/longdesc.found", navigationRequests[0].URL, "could not get correct url")
		})
		t.Run("lowsrc", func(t *testing.T) {
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img lowsrc="/test/html/body/img/lowsrc.found">`))
			resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			navigationRequests := bodyImgTagParser(resp)
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/img/lowsrc.found", navigationRequests[0].URL, "could not get correct url")
		})
		t.Run("src", func(t *testing.T) {
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img src="/test/html/body/img/src.found">`))
			resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			navigationRequests := bodyImgTagParser(resp)
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/img/src.found", navigationRequests[0].URL, "could not get correct url")
		})
		t.Run("srcset", func(t *testing.T) {
			var gotURL []string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img srcset="/test/html/body/img/srcset1x.found 1x,
				/test/html/body/img/srcset2x.found 2x">`))
			resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			for _, navigationResponse := range bodyImgTagParser(resp) {
				gotURL = append(gotURL, navigationResponse.URL)
			}
			require.ElementsMatch(t, []string{
				"https://security-crawl-maze.app/test/html/body/img/srcset1x.found",
				"https://security-crawl-maze.app/test/html/body/img/srcset2x.found",
			}, gotURL, "could not get correct url")
		})
	})
	t.Run("html-body", func(t *testing.T) {
		// TODO: Fix parsing
		//
		// parsed, _ = url.Parse("https://security-crawl-maze.app/html/body/frameset/frame/src.html")
		// var gotURL []string
		// resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Body: []byte(`<p>
		// 	The test contains an inline string with known extension - /string-known-extension.pdf
		// 	The test contains an inline string - ./test/html/misc/string/dot-slash-prefix.found
		// 	The test contains an inline string - ../test/html/misc/string/dot-dot-slash-prefix.found
		// 	The test contains an inline string - http://security-crawl-maze.app/test/html/misc/string/url-string.found
		//   </p>`), Options: &types.CrawlerOptions{Options: &types.Options{ScrapeJSResponses: true}}}
		// bodyScrapeEndpointsParser(resp, func(resp navigation.Request) {
		// 	gotURL = append(gotURL, resp.URL)
		// })
		// require.ElementsMatch(t, []string{
		// 	"https://security-crawl-maze.app/test/string-known-extension.pdf",
		// 	"https://security-crawl-maze.app/test/html/misc/string/dot-slash-prefix.found",
		// 	"https://security-crawl-maze.app/test/html/misc/string/dot-dot-slash-prefix.found",
		// 	"http://security-crawl-maze.app/test/html/misc/string/url-string.found",
		// }, gotURL, "could not get correct url")
	})
	t.Run("object", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<object data="/test/html/body/object/data.found"></object>`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyObjectTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/object/data.found", navigationRequests[0].URL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<object codebase="/test/html/body/object/codebase.found"></object>`))
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests = bodyObjectTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/object/codebase.found", navigationRequests[0].URL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<object classid="clsid:6BF52A52-394A-11d3-B153-00C04F79FAA6">
		<param name="ref" value="/test/html/body/object/param/value.found"></param>
	  </object>`))
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests = bodyObjectTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/object/param/value.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("svg", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">
		<image xlink:href="/test/html/body/svg/image/xlink.found"/>
	  </svg>`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodySvgTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/svg/image/xlink.found", navigationRequests[0].URL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">
		<script xlink:href="/test/html/body/svg/script/xlink.found"></script>
	  </svg>`))
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests = bodySvgTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/svg/script/xlink.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("table", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<table background="/test/html/body/table/background.found"></table>`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyTableTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/table/background.found", navigationRequests[0].URL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<table>
		<tr>
			<td background="/test/html/body/table/td/background.found"></td>
		</tr>
	</table>`))
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests = bodyTableTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/table/td/background.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("video", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<video poster="/test/html/body/video/poster.found"></video>`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyVideoTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/video/poster.found", navigationRequests[0].URL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<video src="/test/html/body/video/src.found"></video>`))
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests = bodyVideoTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/video/src.found", navigationRequests[0].URL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<video width="320" height="240" controls>
		<track src="/test/html/body/video/track/src.found" kind="subtitles" srclang="en" label="English">
	</video>`))
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests = bodyVideoTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/video/track/src.found", navigationRequests[0].URL, "could not get correct url")

	})
	t.Run("applet", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<applet archive="/test/html/body/applet/archive.found"></applet>`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyAppletTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/applet/archive.found", navigationRequests[0].URL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<applet code = "Test" codebase="/test/html/body/applet/codebase.found"></applet>`))
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests = bodyAppletTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/applet/codebase.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("link", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<link rel=\"stylesheet\" href=\"/css/font-face.css\">"))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyLinkHrefTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/css/font-face.css", navigationRequests[0].URL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<link rel="prefetch" href="/test/html/head/link/href.found" />`))
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests = bodyLinkHrefTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/head/link/href.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("base", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<base href="/test/html/head/base/href.found">`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyBaseHrefTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/head/base/href.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("manifest", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<html xmlns="http://www.w3.org/1999/xhtml" manifest="/test/html/manifest.found">`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyHtmlManifestTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/manifest.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("doctype", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<!DOCTYPE html SYSTEM "/test/html/doctype.found">
<meta charset="utf-8">`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyHtmlDoctypeTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/doctype.found", navigationRequests[0].URL, "could not get correct url")

	})
	t.Run("import", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<IMPORT namespace="myNS" implementation="/test/html/head/import/implementation.found" /></IMPORT>`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyImportImplementationTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/head/import/implementation.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("embed", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<embed src=\"/test/html/body/embed/src.found\"></embed>"))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyEmbedTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/embed/src.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("frame", func(t *testing.T) {
		//	var gotURL string
		//	// TODO: Fix test
		//	documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`
		//	<!DOCTYPE html>
		//	<meta charset="utf-8">
		//	<title>CrawlMaze - Testbed for Web Crawlers - frame tag</title>
		//	<h1>src attribute</h1>
		//	<frameset>
		//	  <frame src="/test/html/body/frameset/frame/src.found"></frame>
		//	</frameset>`))
		//	resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		//	bodyFrameTagParser(resp, func(resp navigation.Request) {
		//		gotURL = resp.URL
		//	})
		//	require.Equal(t, "https://security-crawl-maze.app/test/html/body/frameset/frame/src.found", gotURL, "could not get correct url")
	})
	t.Run("iframe", func(t *testing.T) {
		t.Run("src", func(t *testing.T) {
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<iframe src=\"/test/html/body/iframe/src.found\"></iframe>"))
			resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			navigationRequests := bodyIframeTagParser(resp)
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/iframe/src.found", navigationRequests[0].URL, "could not get correct url")
		})
		t.Run("srcdoc", func(t *testing.T) {
			//var gotURL string
			//documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<iframe srcdoc=\"<img src=/test/html/body/iframe/srcdoc.found>\"></iframe>"))
			//resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			//bodyIframeTagParser(resp, func(resp navigation.Request) {
			//	gotURL = resp.URL
			//})
			//require.Equal(t, "https://security-crawl-maze.app/test/html/body/iframe/srcdoc.found", gotURL, "could not get correct url")
		})
	})
	t.Run("input", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<input type=\"image\" src=\"/test/html/body/input/src.found\" name=\"test\" value=\"test\">"))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyInputSrcTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/input/src.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("isindex", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<isindex action=\"/test/html/body/isindex/action.found\"></isindex>"))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyIsindexActionTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/isindex/action.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("script", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<script src=\"/test/html/body/script/src.found\"></script>"))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyScriptSrcTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/script/src.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("button", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<form id=\"test\"><button form=\"test\" formaction=\"/test/html/body/form/button/formaction.found\" type=\"submit\">CLICKME</button></form>"))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyButtonFormactionTagParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/form/button/formaction.found", navigationRequests[0].URL, "could not get correct url")
	})
	t.Run("form", func(t *testing.T) {
		t.Run("get", func(t *testing.T) {
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<form action=\"/test/html/body/form/action-get.found\" method=\"GET\"><input type=\"text\" name=\"test1\" value=\"test\"><input type=\"text\" name=\"test2\" value=\"test\"></form>"))
			resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			navigationRequests := bodyFormTagParser(resp)
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/form/action-get.found?test1=test&test2=test", navigationRequests[0].URL, "could not get correct url")
		})
		t.Run("post", func(t *testing.T) {
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<form action=\"/test/html/body/form/action-post.found\" method=\"POST\" enctype=\"multipart/form-data\"><input type=\"text\" name=\"test1\" value=\"test\"><input type=\"text\" name=\"test2\" value=\"test\"></form>"))
			resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
			navigationRequests := bodyFormTagParser(resp)
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/form/action-post.found", navigationRequests[0].URL, "could not get correct url")
			require.Equal(t, "POST", navigationRequests[0].Method, "could not get correct method")
		})
	})

	t.Run("meta", func(t *testing.T) {
		//	var gotURL string
		//	documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<meta http-equiv=\"refresh\" content=\"10; url=/test/html/head/meta/content-redirect.found\">"))
		//	resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		//	bodyMetaContentTagParser(resp, func(resp navigation.Request) {
		//		gotURL = resp.URL
		//	})
		//	require.Equal(t, "https://security-crawl-maze.app/test/html/head/meta/content-redirect.found", gotURL, "could not get correct url")
		//
		//	documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<meta http-equiv="Content-Security-Policy" content="script-src 'self'; report-uri /test/html/head/meta/content-csp.found">`))
		//	resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		//	bodyMetaContentTagParser(resp, func(resp navigation.Request) {
		//		gotURL = resp.URL
		//	})
		//	require.Equal(t, "https://security-crawl-maze.app/test/html/head/meta/content-csp.found", gotURL, "could not get correct url")
		//
		//	documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<meta name="msapplication-config" content="/test/html/head/meta/content-pinned-websites.found">`))
		//	resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		//	bodyMetaContentTagParser(resp, func(resp navigation.Request) {
		//		gotURL = resp.URL
		//	})
		//	require.Equal(t, "https://security-crawl-maze.app/test/html/head/meta/content-pinned-websites.found", gotURL, "could not get correct url")
		//
		//	documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<meta name="copyright" content="<img src='/test/html/head/meta/content-reading-view.found'>">`))
		//	resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		//	bodyMetaContentTagParser(resp, func(resp navigation.Request) {
		//		gotURL = resp.URL
		//	})
		//	require.Equal(t, "https://security-crawl-maze.app/test/html/head/meta/content-reading-view.found", gotURL, "could not get correct url")
	})
}

func TestScriptParsers(t *testing.T) {
	parsed, _ := urlutil.Parse("https://security-crawl-maze.app/html/script/xyz/")

	t.Run("content", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<script>var endpoint='/test/html/script/content.do';</script>"))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := scriptContentRegexParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/script/content.do", navigationRequests[0].URL, "could not get correct url")
	})

	t.Run("js", func(t *testing.T) {
		parsed, _ = urlutil.Parse("https://security-crawl-maze.app/html/script/xyz/data.js")
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Body: "var endpoint='/test/html/script/body.do';"}
		navigationRequests := scriptJSFileRegexParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/script/body.do", navigationRequests[0].URL, "could not get correct url")

		parsed, _ = urlutil.Parse("https://security-crawl-maze.app/html/script/xyz/")
		resp = &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}, Header: http.Header{"Content-Type": []string{"application/javascript"}}}, Body: "var endpoint='/test/html/script/body-content-type.do';"}
		navigationRequests = scriptJSFileRegexParser(resp)
		require.Equal(t, "https://security-crawl-maze.app/test/html/script/body-content-type.do", navigationRequests[0].URL, "could not get correct url")

	})
}

func TestRegexBodyParsers(t *testing.T) {
	parsed, _ := urlutil.Parse("https://security-crawl-maze.app/contact")
	t.Run("regexbody", func(t *testing.T) {
		output.CustomFieldsMap = make(map[string]output.CustomFieldConfig)
		resp := &navigation.Response{
			Resp:  &http.Response{Request: &http.Request{URL: parsed.URL}},
			Depth: 0,
			Body:  "some content contact@example.com",
		}

		// set required regex
		output.CustomFieldsMap["email"] = output.CustomFieldConfig{
			Name:         "email",
			Type:         "regex",
			Part:         "body",
			CompileRegex: []*regexp.Regexp{regexp.MustCompile(`([a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+\.[a-zA-Z0-9_-]+)`)},
		}

		navigationRequests := customFieldRegexParser(resp)
		var requireFields = map[string][]string{"email": {"contact@example.com"}}
		require.Equal(t, requireFields, navigationRequests[0].CustomFields, "could not get correct url")
	})
	t.Run("regexheader", func(t *testing.T) {
		output.CustomFieldsMap = make(map[string]output.CustomFieldConfig)
		resp := &navigation.Response{
			Resp: &http.Response{Request: &http.Request{URL: parsed.URL},
				Header: http.Header{
					"server": []string{"ECS (dcb/7F84)"},
				},
			},
		}

		// set required regex
		output.CustomFieldsMap["server"] = output.CustomFieldConfig{
			Name:         "server",
			Type:         "regex",
			Part:         "header",
			CompileRegex: []*regexp.Regexp{regexp.MustCompile(`server: ECS`)},
		}

		navigationRequests := customFieldRegexParser(resp)
		var requireFields = map[string][]string{"server": {"server: ECS"}}
		require.Equal(t, requireFields, navigationRequests[0].CustomFields, "could not get correct url")
	})

	t.Run("regexresponse", func(t *testing.T) {
		output.CustomFieldsMap = make(map[string]output.CustomFieldConfig)
		resp := &navigation.Response{
			Resp: &http.Response{Request: &http.Request{URL: parsed.URL},
				Header: http.Header{
					"server": []string{"ECS (dcb/7F84)"},
				},
			},
			Body: "some content contact@example.com",
		}

		// set required regex
		output.CustomFieldsMap["server"] = output.CustomFieldConfig{
			Name:         "server",
			Type:         "regex",
			Part:         "response",
			CompileRegex: []*regexp.Regexp{regexp.MustCompile(`ECS`)},
		}
		output.CustomFieldsMap["email"] = output.CustomFieldConfig{
			Name:         "email",
			Type:         "regex",
			Part:         "response",
			CompileRegex: []*regexp.Regexp{regexp.MustCompile(`([a-zA-Z0-9._-]+@[a-zA-Z0-9._-]+\.[a-zA-Z0-9_-]+)`)},
		}

		navigationRequests := customFieldRegexParser(resp)
		var requireFields = map[string][]string{"server": {"ECS"}, "email": {"contact@example.com"}}
		require.Equal(t, requireFields, navigationRequests[0].CustomFields, "could not get correct url")
	})
}

func TestHtmxBodyParser(t *testing.T) {
	parsed, _ := urlutil.Parse("https://htmx.org/examples/")

	t.Run("hx-get", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<button hx-get="/contact/1/edit" class="btn primary">Click To Edit</button>`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyHtmxAttrParser(resp)
		require.Equal(t, "https://htmx.org/contact/1/edit", navigationRequests[0].URL, "could not get correct url")
		require.Equal(t, "GET", navigationRequests[0].Method, "could not get correct method")
	})
	t.Run("hx-post", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<form id="checked-contacts" hx-post="/users" hx-swap="outerHTML settle:3s" hx-target="#toast">`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyHtmxAttrParser(resp)
		require.Equal(t, "https://htmx.org/users", navigationRequests[0].URL, "could not get correct url")
		require.Equal(t, "POST", navigationRequests[0].Method, "could not get correct method")
	})
	t.Run("hx-put", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<button hx-put="/account" hx-target="body">`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyHtmxAttrParser(resp)
		require.Equal(t, "https://htmx.org/account", navigationRequests[0].URL, "could not get correct url")
		require.Equal(t, "PUT", navigationRequests[0].Method, "could not get correct method")

	})
	t.Run("hx-patch", func(t *testing.T) {
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<button hx-patch="/account" hx-target="body">`))
		resp := &navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed.URL}}, Reader: documentReader}
		navigationRequests := bodyHtmxAttrParser(resp)
		require.Equal(t, "https://htmx.org/account", navigationRequests[0].URL, "could not get correct url")
		require.Equal(t, "PATCH", navigationRequests[0].Method, "could not get correct method")
	})
}
