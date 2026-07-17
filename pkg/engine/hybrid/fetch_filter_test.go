package hybrid

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestShouldInspectFetchResource(t *testing.T) {
	t.Parallel()

	t.Run("document always inspected", func(t *testing.T) {
		t.Parallel()
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeDocument, nil))
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeDocument, &types.Options{}))
	})

	t.Run("skip heavy static assets by default", func(t *testing.T) {
		t.Parallel()
		opts := &types.Options{}
		for _, rt := range []proto.NetworkResourceType{
			proto.NetworkResourceTypeImage,
			proto.NetworkResourceTypeFont,
			proto.NetworkResourceTypeMedia,
			proto.NetworkResourceTypePing,
			proto.NetworkResourceTypePrefetch,
			proto.NetworkResourceTypeWebSocket,
			proto.NetworkResourceTypeEventSource,
			proto.NetworkResourceTypeManifest,
			proto.NetworkResourceTypeCSPViolationReport,
			proto.NetworkResourceTypeOther,
		} {
			require.Falsef(t, shouldInspectFetchResource(rt, opts), "expected skip for %s", rt)
		}
	})

	t.Run("script gated by js or xhr flags", func(t *testing.T) {
		t.Parallel()
		require.False(t, shouldInspectFetchResource(proto.NetworkResourceTypeScript, &types.Options{}))
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeScript, &types.Options{ScrapeJSResponses: true}))
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeScript, &types.Options{ScrapeJSLuiceResponses: true}))
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeScript, &types.Options{XhrExtraction: true}))
	})

	t.Run("stylesheet gated by js crawl", func(t *testing.T) {
		t.Parallel()
		require.False(t, shouldInspectFetchResource(proto.NetworkResourceTypeStylesheet, &types.Options{}))
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeStylesheet, &types.Options{ScrapeJSResponses: true}))
	})

	t.Run("xhr and fetch always inspected", func(t *testing.T) {
		t.Parallel()
		opts := &types.Options{}
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeXHR, opts))
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeFetch, opts))
	})
}
