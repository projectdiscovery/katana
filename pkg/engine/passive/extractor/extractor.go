package extractor

// UrlExtractor is an interface that defines the contract for domain extraction.
type UrlExtractor interface {
	Extract(text string) []string
}
