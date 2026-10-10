package crawler

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCrawlerOptionsPropagation(t *testing.T) {
	c, err := New(Options{
		UseInstalledChrome: true,
		ChromiumPath:       "/mock/chrome",
		MaxBrowsers:        1,
	})
	require.NoError(t, err)
	require.True(t, c.options.UseInstalledChrome)
	require.Equal(t, "/mock/chrome", c.options.ChromiumPath)
}
