package main

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/projectdiscovery/gologger"
	urlutil "github.com/projectdiscovery/utils/url"
)

// expectedResults is the list of expected endpoints from security-crawl-maze
// blueprint directory.
// https://github.com/google/security-crawl-maze/blob/master/blueprints/utils/resources/expected-results.json
var expectedResults = []string{
	"/css/font-face.found",
	"/headers/content-location.found",
	"/headers/link.found",
	"/headers/location.found",
	"/headers/refresh.found",
	"/html/doctype.found",
	"/html/manifest.found",
	"/html/body/background.found",
	"/html/body/a/href.found",
	"/html/body/a/ping.found",
	"/html/body/audio/src.found",
	"/html/body/audio/source/src.found",
	"/html/body/audio/source/srcset1x.found",
	"/html/body/audio/source/srcset2x.found",
	"/html/body/applet/archive.found",
	"/html/body/applet/codebase.found",
	"/html/body/blockquote/cite.found",
	"/html/body/embed/src.found",
	"/html/body/form/action-get.found",
	"/html/body/form/action-post.found",
	"/html/body/form/button/formaction.found",
	"/html/body/frameset/frame/src.found",
	"/html/body/iframe/src.found",
	"/html/body/iframe/srcdoc.found",
	"/html/body/img/dynsrc.found",
	"/html/body/img/lowsrc.found",
	"/html/body/img/longdesc.found",
	"/html/body/img/src-data.found",
	"/html/body/img/src.found",
	"/html/body/img/srcset1x.found",
	"/html/body/img/srcset2x.found",
	"/html/body/input/src.found",
	"/html/body/isindex/action.found",
	"/html/body/map/area/ping.found",
	"/html/body/object/data.found",
	"/html/body/object/codebase.found",
	"/html/body/object/param/value.found",
	"/html/body/script/src.found",
	"/html/body/svg/image/xlink.found",
	"/html/body/svg/script/xlink.found",
	"/html/body/table/background.found",
	"/html/body/table/td/background.found",
	"/html/body/video/src.found",
	"/html/body/video/track/src.found",
	"/html/body/video/poster.found",
	"/html/head/profile.found",
	"/html/head/base/href.found",
	"/html/head/comment-conditional.found",
	"/html/head/import/implementation.found",
	"/html/head/link/href.found",
	"/html/head/meta/content-csp.found",
	"/html/head/meta/content-pinned-websites.found",
	"/html/head/meta/content-reading-view.found",
	"/html/head/meta/content-redirect.found",
	"/html/misc/url/full-url.found",
	"/html/misc/url/path-relative-url.found",
	"/html/misc/url/protocol-relative-url.found",
	"/html/misc/url/root-relative-url.found",
	"/html/misc/string/dot-dot-slash-prefix.found",
	"/html/misc/string/dot-slash-prefix.found",
	"/html/misc/string/url-string.found",
	"/html/misc/string/string-known-extension.pdf",
	"/javascript/misc/automatic-post.found",
	"/javascript/misc/comment.found",
	"/javascript/misc/string-variable.found",
	"/javascript/misc/string-concat-variable.found",
	"/javascript/frameworks/angular/event-handler.found",
	"/javascript/frameworks/angular/router-outlet.found",
	"/javascript/frameworks/angularjs/ng-href.found",
	"/javascript/frameworks/polymer/event-handler.found",
	"/javascript/frameworks/polymer/polymer-router.found",
	"/javascript/frameworks/react/route-path.found",
	"/javascript/frameworks/react/index.html/search.found",
	"/javascript/interactive/js-delete.found",
	"/javascript/interactive/js-post.found",
	"/javascript/interactive/js-post-event-listener.found",
	"/javascript/interactive/js-put.found",
	"/javascript/interactive/listener-and-event-attribute-first.found",
	"/javascript/interactive/listener-and-event-attribute-second.found",
	"/javascript/interactive/multi-step-request-event-attribute.found",
	"/test/javascript/interactive/multi-step-request-event-listener-div-dom.found",
	"/test/javascript/interactive/multi-step-request-event-listener-div.found",
	"/javascript/interactive/multi-step-request-event-listener-dom.found",
	"/javascript/interactive/multi-step-request-event-listener.found",
	"/javascript/interactive/multi-step-request-redefine-event-attribute.found",
	"/javascript/interactive/multi-step-request-remove-button.found",
	"/javascript/interactive/multi-step-request-remove-event-listener.found",
	"/javascript/interactive/two-listeners-first.found",
	"/javascript/interactive/two-listeners-second.found",
	"/misc/known-files/robots.txt.found",
	"/misc/known-files/sitemap.xml.found",
}

func main() {
	if err := process(); err != nil {
		log.Fatalf("%s\n", err)
	}
}

var urlTestPrefix = "/test"

func process() error {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: crawl-maze-score output.txt output_headless.txt")
		return nil
	}
	input := os.Args[1]
	inputHeadless := os.Args[2]

	links, err := readFoundLinks(input)
	if err != nil {
		return err
	}
	linksHeadless, err := readFoundLinks(inputHeadless)
	if err != nil {
		return err
	}

	linksMap := make(map[string]struct{})
	linksHeadlessMap := make(map[string]struct{})
	for _, link := range links {
		linksMap[link] = struct{}{}
	}
	for _, link := range linksHeadless {
		linksHeadlessMap[link] = struct{}{}
	}
	matches, matchesHeadless := 0, 0
	for _, expected := range expectedResults {
		expected = urlTestPrefix + expected

		_, normalOk := linksMap[expected]
		_, headlessOk := linksHeadlessMap[expected]

		if normalOk {
			matches++
		}
		if headlessOk {
			matchesHeadless++
		}
		fmt.Printf("[%s] [%s] %s\n", colorizeText("standard", normalOk), colorizeText("headless", headlessOk), expected)
	}
	fmt.Printf("[info] Total links (%d): Standard=>%d Headless=>%d\n", len(expectedResults), len(links), len(linksHeadless))
	fmt.Printf("[info] Total: %d NormalMatches=>%d HeadlessMatches=>%d\n", len(expectedResults), matches, matchesHeadless)
	fmt.Printf("[info] Score: Normal=>%.2f%% Headless=>%.2f%%\n", math.Round(float64(matches*100/len(expectedResults))), math.Round(float64(matchesHeadless*100/len(expectedResults))))
	return nil
}

func colorizeText(text string, value bool) string {
	if value {
		return aurora.Green(text + ":yes").String()
	}
	return aurora.Red(text + ":no").String()
}

func strippedLink(link string) string {
	parsed, err := urlutil.Parse(link)
	if err != nil {
		gologger.Warning().Msgf("failed to parse link while extracting path: %v", err)
	}
	return parsed.Path
}

func readFoundLinks(input string) ([]string, error) {
	file, err := os.Open(input)
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := file.Close(); err != nil {
			gologger.Error().Msgf("Error closing file: %v\n", err)
		}
	}()

	scanner := bufio.NewScanner(file)
	var links []string
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			break
		}
		if strings.Contains(text, ".found") {
			links = append(links, strippedLink(text))
		}
	}
	return links, nil
}
