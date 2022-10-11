package parser

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestHeaderParsers(t *testing.T) {
	parsed, _ := url.Parse("https://security-crawl-maze.app/headers/xyz/")

	t.Run("content-location", func(t *testing.T) {
		var gotURL string
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Content-Location": []string{"/test/headers/content-location.found"}}}}
		headerContentLocationParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/headers/content-location.found", gotURL, "could not get correct url")
	})
	t.Run("link", func(t *testing.T) {
		var gotURL string
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Link": []string{"</test/headers/link.found>; rel=\"preload\""}}}}
		headerLinkParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/headers/link.found", gotURL, "could not get correct url")
	})
	t.Run("location", func(t *testing.T) {
		var gotURL string
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Location": []string{"http://security-crawl-maze.app/test/headers/location.found"}}}}
		headerLocationParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "http://security-crawl-maze.app/test/headers/location.found", gotURL, "could not get correct url")
	})
	t.Run("refresh", func(t *testing.T) {
		var gotURL string
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Refresh": []string{"999; url=/test/headers/refresh.found"}}}}
		headerRefreshParser(resp, func(resp navigation.Request) {
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
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyATagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/a/href.found", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader("<a ping=/test/html/body/a/ping.found>"))
		resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyATagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/a/ping.found", gotURL, "could not get correct url")
	})
	t.Run("background", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<body background=\"/test/html/body/background.found\"></body>"))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyBackgroundTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/background.found", gotURL, "could not get correct url")
	})
	t.Run("blockquote", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<blockquote cite="/test/html/body/blockquote/cite.found"></blockquote>`))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyBlockquoteCiteTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/blockquote/cite.found", gotURL, "could not get correct url")
	})
	t.Run("frameset", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<frameset>
		<frame src="/test/html/body/frameset/frame/src.found"></frame>
	  </frameset>`))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyFrameSrcTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/frameset/frame/src.found", gotURL, "could not get correct url")
	})
	t.Run("area", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<map name="map">
		<area ping="/test/html/body/map/area/ping.found" shape="rect" coords="0,0,150,150" href="#">
	  </map>`))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyMapAreaPingTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/map/area/ping.found", gotURL, "could not get correct url")
	})
	t.Run("audio", func(t *testing.T) {
		t.Run("src", func(t *testing.T) {
			var gotURL string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<audio src=\"/test/html/body/audio/src.found\"></audio>"))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyAudioTagParser(resp, func(resp navigation.Request) {
				gotURL = resp.URL
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/audio/src.found", gotURL, "could not get correct url")
		})
		t.Run("source", func(t *testing.T) {
			t.Run("src", func(t *testing.T) {
				var gotURL string
				documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<audio controls><source src=\"/test/html/body/audio/source/src.found\" type=\"audio/mpeg\"></audio>"))
				resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
				bodyAudioTagParser(resp, func(resp navigation.Request) {
					gotURL = resp.URL
				})
				require.Equal(t, "https://security-crawl-maze.app/test/html/body/audio/source/src.found", gotURL, "could not get correct url")
			})
			t.Run("srcset", func(t *testing.T) {
				var gotURL []string
				documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<audio controls>
				<source srcset="/test/html/body/audio/source/srcset1x.found 1x,
								/test/html/body/audio/source/srcset2x.found 2x">
			</audio>`))
				resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
				bodyAudioTagParser(resp, func(resp navigation.Request) {
					gotURL = append(gotURL, resp.URL)
				})
				require.ElementsMatch(t, []string{
					"https://security-crawl-maze.app/test/html/body/audio/source/srcset1x.found",
					"https://security-crawl-maze.app/test/html/body/audio/source/srcset2x.found",
				}, gotURL, "could not get correct url")
			})
		})
	})
	t.Run("img", func(t *testing.T) {
		t.Run("dynsrc", func(t *testing.T) {
			var gotURL string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img dynsrc="/test/html/body/img/dynsrc.found">`))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyImgTagParser(resp, func(resp navigation.Request) {
				gotURL = resp.URL
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/img/dynsrc.found", gotURL, "could not get correct url")
		})
		t.Run("longdesc", func(t *testing.T) {
			var gotURL string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img alt="" src="#" longdesc="/test/html/body/img/longdesc.found">`))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyImgTagParser(resp, func(resp navigation.Request) {
				gotURL = resp.URL
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/img/longdesc.found", gotURL, "could not get correct url")
		})
		t.Run("lowsrc", func(t *testing.T) {
			var gotURL string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img lowsrc="/test/html/body/img/lowsrc.found">`))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyImgTagParser(resp, func(resp navigation.Request) {
				gotURL = resp.URL
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/img/lowsrc.found", gotURL, "could not get correct url")
		})
		t.Run("src", func(t *testing.T) {
			var gotURL string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img src="/test/html/body/img/src.found">`))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyImgTagParser(resp, func(resp navigation.Request) {
				gotURL = resp.URL
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/img/src.found", gotURL, "could not get correct url")
		})
		t.Run("srcset", func(t *testing.T) {
			var gotURL []string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<img srcset="/test/html/body/img/srcset1x.found 1x,
				/test/html/body/img/srcset2x.found 2x">`))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyImgTagParser(resp, func(resp navigation.Request) {
				gotURL = append(gotURL, resp.URL)
			})
			require.ElementsMatch(t, []string{
				"https://security-crawl-maze.app/test/html/body/img/srcset1x.found",
				"https://security-crawl-maze.app/test/html/body/img/srcset2x.found",
			}, gotURL, "could not get correct url")
		})
	})
	t.Run("object", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<object data="/test/html/body/object/data.found"></object>`))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyObjectTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/object/data.found", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<object codebase="/test/html/body/object/codebase.found"></object>`))
		resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyObjectTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/object/codebase.found", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<object classid="clsid:6BF52A52-394A-11d3-B153-00C04F79FAA6">
		<param name="ref" value="/test/html/body/object/param/value.found"></param>
	  </object>`))
		resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyObjectTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/object/param/value.found", gotURL, "could not get correct url")
	})
	t.Run("svg", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">
		<image xlink:href="/test/html/body/svg/image/xlink.found"/>
	  </svg>`))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodySvgTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/svg/image/xlink.found", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink">
		<script xlink:href="/test/html/body/svg/script/xlink.found"></script>
	  </svg>`))
		resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodySvgTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/svg/script/xlink.found", gotURL, "could not get correct url")
	})
	t.Run("table", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<table background="/test/html/body/table/background.found"></table>`))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyTableTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/table/background.found", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<table>
		<tr>
			<td background="/test/html/body/table/td/background.found"></td>
		</tr>
	</table>`))
		resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyTableTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/table/td/background.found", gotURL, "could not get correct url")
	})
	t.Run("video", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<video poster="/test/html/body/video/poster.found"></video>`))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyVideoTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/video/poster.found", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<video src="/test/html/body/video/src.found"></video>`))
		resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyVideoTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/video/src.found", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<video width="320" height="240" controls>
		<track src="/test/html/body/video/track/src.found" kind="subtitles" srclang="en" label="English">
	</video>`))
		resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyVideoTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/video/track/src.found", gotURL, "could not get correct url")

	})
	t.Run("applet", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<applet archive="/test/html/body/applet/archive.found"></applet>`))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyAppletTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/applet/archive.found", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<applet code = "Test" codebase="/test/html/body/applet/codebase.found"></applet>`))
		resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyAppletTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/applet/codebase.found", gotURL, "could not get correct url")
	})
	t.Run("link", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<link rel=\"stylesheet\" href=\"/css/font-face.css\">"))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyLinkHrefTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/css/font-face.css", gotURL, "could not get correct url")

		documentReader, _ = goquery.NewDocumentFromReader(strings.NewReader(`<link rel="prefetch" href="/test/html/head/link/href.found" />`))
		resp = navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyLinkHrefTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/head/link/href.found", gotURL, "could not get correct url")
	})
	t.Run("base", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader(`<base href="/test/html/head/base/href.found">`))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyBaseHrefTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/head/base/href.found", gotURL, "could not get correct url")
	})
	t.Run("embed", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<embed src=\"/test/html/body/embed/src.found\"></embed>"))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyEmbedTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/embed/src.found", gotURL, "could not get correct url")
	})
	t.Run("frame", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<frameset><frame src=\"/test/html/body/frameset/frame/src.found\"></frame></frameset>"))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyFrameTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/frameset/frame/src.found", gotURL, "could not get correct url")
	})
	t.Run("iframe", func(t *testing.T) {
		t.Run("src", func(t *testing.T) {
			var gotURL string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<iframe src=\"/test/html/body/iframe/src.found\"></iframe>"))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyIframeTagParser(resp, func(resp navigation.Request) {
				gotURL = resp.URL
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/iframe/src.found", gotURL, "could not get correct url")
		})
		t.Run("srcdoc", func(t *testing.T) {
			var gotURL string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<iframe srcdoc=\"<img src=/test/html/body/iframe/srcdoc.found>\"></iframe>"))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyIframeTagParser(resp, func(resp navigation.Request) {
				gotURL = resp.URL
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/iframe/srcdoc.found", gotURL, "could not get correct url")
		})
	})
	t.Run("input", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<input type=\"image\" src=\"/test/html/body/input/src.found\" name=\"test\" value=\"test\">"))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyInputSrcTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/input/src.found", gotURL, "could not get correct url")
	})
	t.Run("isindex", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<isindex action=\"/test/html/body/isindex/action.found\"></isindex>"))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyIsindexActionTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/isindex/action.found", gotURL, "could not get correct url")
	})
	t.Run("script", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<script src=\"/test/html/body/script/src.found\"></script>"))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyScriptSrcTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/script/src.found", gotURL, "could not get correct url")
	})
	t.Run("button", func(t *testing.T) {
		var gotURL string
		documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<form id=\"test\"><button form=\"test\" formaction=\"/test/html/body/form/button/formaction.found\" type=\"submit\">CLICKME</button></form>"))
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyButtonFormactionTagParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/body/form/button/formaction.found", gotURL, "could not get correct url")
	})
	t.Run("form", func(t *testing.T) {
		t.Run("get", func(t *testing.T) {
			var gotURL string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<form action=\"/test/html/body/form/action-get.found\" method=\"GET\"><input type=\"text\" name=\"test1\" value=\"test\"><input type=\"text\" name=\"test2\" value=\"test\"></form>"))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyFormTagParser(resp, func(resp navigation.Request) {
				gotURL = resp.URL
			})
			require.Equal(t, "https://security-crawl-maze.app/test/html/body/form/action-get.found?test1=test&test2=test", gotURL, "could not get correct url")
		})
		t.Run("post", func(t *testing.T) {
			var gotURL string
			var method string
			documentReader, _ := goquery.NewDocumentFromReader(strings.NewReader("<form action=\"/test/html/body/form/action-post.found\" method=\"POST\" enctype=\"multipart/form-data\"><input type=\"text\" name=\"test1\" value=\"test\"><input type=\"text\" name=\"test2\" value=\"test\"></form>"))
			resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
			bodyFormTagParser(resp, func(resp navigation.Request) {
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
		resp := navigation.Response{Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		bodyMetaContentTagParser(resp, func(resp navigation.Request) {
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
		resp := navigation.Response{Options: &types.CrawlerOptions{Options: &types.Options{ScrapeJSResponses: true}}, Resp: &http.Response{Request: &http.Request{URL: parsed}}, Reader: documentReader}
		scriptContentRegexParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/script/content.do", gotURL, "could not get correct url")
	})

	t.Run("js", func(t *testing.T) {
		parsed, _ = url.Parse("https://security-crawl-maze.app/html/script/xyz/data.js")
		var gotURL string
		resp := navigation.Response{Options: &types.CrawlerOptions{Options: &types.Options{ScrapeJSResponses: true}}, Resp: &http.Response{Request: &http.Request{URL: parsed}}, Body: []byte("var endpoint='/test/html/script/body.do';")}
		scriptJSFileRegexParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/script/body.do", gotURL, "could not get correct url")

		parsed, _ = url.Parse("https://security-crawl-maze.app/html/script/xyz/")
		gotURL = ""
		resp = navigation.Response{Options: &types.CrawlerOptions{Options: &types.Options{ScrapeJSResponses: true}}, Resp: &http.Response{Request: &http.Request{URL: parsed}, Header: http.Header{"Content-Type": []string{"application/javascript"}}}, Body: []byte("var endpoint='/test/html/script/body-content-type.do';")}
		scriptJSFileRegexParser(resp, func(resp navigation.Request) {
			gotURL = resp.URL
		})
		require.Equal(t, "https://security-crawl-maze.app/test/html/script/body-content-type.do", gotURL, "could not get correct url")

	})
}
