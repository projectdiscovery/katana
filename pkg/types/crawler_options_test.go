package types

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConcurrentConfigureOutput(t *testing.T) {
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			opts := &Options{
				Silent:  idx%3 == 0,
				Verbose: idx%3 == 1,
				Debug:   idx%3 == 2,
			}
			opts.ConfigureOutput()
		}(i)
	}

	wg.Wait()
}

func TestConcurrentNewCrawlerOptions(t *testing.T) {
	const goroutines = 50
	var wg sync.WaitGroup
	wg.Add(goroutines)
	errChan := make(chan error, goroutines)

	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			opts := &Options{
				MaxDepth:     1,
				BodyReadSize: 1024,
				Silent:       idx%3 == 0,
				Verbose:      idx%3 == 1,
				Debug:        idx%3 == 2,
			}
			co, err := NewCrawlerOptions(opts)
			if err != nil {
				errChan <- err
				return
			}
			_ = co.Close()
		}(i)
	}

	wg.Wait()
	close(errChan)

	for err := range errChan {
		require.NoError(t, err)
	}
}
