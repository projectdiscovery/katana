package navigation

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMetaRefreshParsingW23(t *testing.T) {
	tag := "5; url=https://example.com/target"
	parts := strings.Split(tag, ";")
	assert.Equal(t, 2, len(parts))
	delay := strings.TrimSpace(parts[0])
	assert.Equal(t, "5", delay)
}
