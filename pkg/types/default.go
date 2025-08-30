package types

import "github.com/projectdiscovery/katana/pkg/utils/queue"

var DefaultOptions Options

func init() {
	DefaultOptions = Options{
		MaxDepth:     3,
		BodyReadSize: 4 * 1024 * 1024, // 4MB
		Timeout:      10,
		TimeStable:   1,
		Retries:      1,
		Strategy:     queue.DepthFirst.String(),
		FieldScope:   "rdn",

		Concurrency: 10,
		Parallelism: 10,
		RateLimit:   150,
	}
}
