package files

import (
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/retryablehttp-go"
)

type visitFunc func(URL string, callback func(navigation.Request)) error

type KnownFiles struct {
	parsers    []visitFunc
	httpclient *retryablehttp.Client
}

// New returns a new known files parser instance
func New(httpclient *retryablehttp.Client) *KnownFiles {
	parser := &KnownFiles{
		httpclient: httpclient,
	}
	robotsTxtCrawler := &robotsTxtCrawler{httpclient: httpclient}
	sitemapXmlCrawler := &sitemapXmlCrawler{httpclient: httpclient}
	parser.parsers = append(parser.parsers, robotsTxtCrawler.Visit, sitemapXmlCrawler.Visit)
	return parser
}

// Request requests all known files with visitors
func (k *KnownFiles) Request(URL string, callback func(nr navigation.Request)) error {
	for _, visitor := range k.parsers {
		if err := visitor(URL, callback); err != nil {
			return err
		}
	}
	return nil
}
