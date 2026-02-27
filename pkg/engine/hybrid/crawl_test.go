package hybrid

import (
	"errors"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/stretchr/testify/require"
)

func TestShouldFallbackOnDOMError(t *testing.T) {
	require.True(t, shouldFallbackOnDOMError(errors.New("context deadline exceeded <- could not get dom")))
	require.True(t, shouldFallbackOnDOMError(errors.New("CONTEXT DEADLINE EXCEEDED")))
	require.False(t, shouldFallbackOnDOMError(errors.New("navigation failed")))
	require.False(t, shouldFallbackOnDOMError(nil))
}

func TestFallbackBodyFromResponse(t *testing.T) {
	body, ok := fallbackBodyFromResponse(&navigation.Response{Body: "<html>ok</html>"})
	require.True(t, ok)
	require.Equal(t, "<html>ok</html>", body)

	_, ok = fallbackBodyFromResponse(&navigation.Response{})
	require.False(t, ok)

	_, ok = fallbackBodyFromResponse(nil)
	require.False(t, ok)
}
