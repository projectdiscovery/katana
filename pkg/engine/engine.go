package engine

type Engine interface {
	Crawl(string) error
	Close() error
}
