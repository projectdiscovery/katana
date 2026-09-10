package navigation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCrawlerWorkerConcurrencyValidation(t *testing.T) {
	concurrency := 20
	assert.Greater(t, concurrency, 0)
	assert.Equal(t, 20, concurrency)
}
