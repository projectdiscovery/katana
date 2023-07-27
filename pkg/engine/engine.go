package engine

type Engine interface {
	Crawl(string) error
	GetInFlightUrls() []string
	Close() error
}
