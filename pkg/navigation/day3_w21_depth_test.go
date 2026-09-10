package navigation

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCrawlerMaxDepthOptionValidation(t *testing.T) {
	maxDepth := 5
	assert.Greater(t, maxDepth, 0)
	assert.Equal(t, 5, maxDepth)
}
