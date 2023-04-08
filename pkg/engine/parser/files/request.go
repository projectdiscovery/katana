package files

import (
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/retryablehttp-go"
)

type visitFunc func(URL string) ([]*navigation.Request, error)

type KnownFiles struct {
	parsers    []visitFunc
	httpclient *retryablehttp.Client
}

// New returns a new known files parser instance
func New(httpclient *retryablehttp.Client, files string) *KnownFiles {
	parser := &KnownFiles{
		httpclient: httpclient,
	}
	switch files {
	case "robotstxt":
		crawler := &robotsTxtCrawler{httpclient: httpclient}
		parser.parsers = append(parser.parsers, crawler.Visit)
	case "sitemapxml":
		crawler := &sitemapXmlCrawler{httpclient: httpclient}
		parser.parsers = append(parser.parsers, crawler.Visit)
	default:
		crawler := &robotsTxtCrawler{httpclient: httpclient}
		parser.parsers = append(parser.parsers, crawler.Visit)
		another := &sitemapXmlCrawler{httpclient: httpclient}
		parser.parsers = append(parser.parsers, another.Visit)
	}
	return parser
}

// Request requests all known files with visitors
func (k *KnownFiles) Request(URL string) (navigationRequests []*navigation.Request, err error) {
	for _, visitor := range k.parsers {
		navRequests, err := visitor(URL)
		if err != nil {
			return navigationRequests, err
		}
		navigationRequests = append(navigationRequests, navRequests...)
	}
	return
}
