//go:build !jsluice || windows || 386

package parser

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitWithOptionsWithoutJSLuiceTagSkipsJSLuiceParsers(t *testing.T) {
	responseParser := NewResponseParser()
	baseParsers := len(*responseParser)

	responseParser.InitWithOptions(&Options{
		ScrapeJSLuiceResponses: true,
		DisableRedirects:       true,
	})

	require.Equal(t, baseParsers, len(*responseParser))
}
