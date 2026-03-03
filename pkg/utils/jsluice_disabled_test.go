//go:build !jsluice || windows || 386

package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtractJsluiceEndpointsDisabledWithoutBuildTag(t *testing.T) {
	endpoints := ExtractJsluiceEndpoints(`fetch("/api/v1/users")`)
	require.Empty(t, endpoints)
}
