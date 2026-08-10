package hybrid

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestHybridBrowserAgents(t *testing.T) {
	t.Parallel()
	require.Equal(t, 1, hybridBrowserAgents(nil))
	require.Equal(t, 1, hybridBrowserAgents(&types.Options{Concurrency: 0}))
	require.Equal(t, 4, hybridBrowserAgents(&types.Options{Concurrency: 4}))
	require.Equal(t, maxHybridBrowserAgents, hybridBrowserAgents(&types.Options{Concurrency: 100}))
	require.Equal(t, 1, hybridBrowserAgents(&types.Options{Concurrency: 10, ChromeWSUrl: "ws://127.0.0.1:9222"}))
	require.Equal(t, 1, hybridBrowserAgents(&types.Options{Concurrency: 10, ChromeDataDir: "/tmp/chrome"}))
}

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
			proto.NetworkResourceTypeXHR,
			proto.NetworkResourceTypeFetch,
			proto.NetworkResourceTypeScript,
			proto.NetworkResourceTypeStylesheet,
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

	t.Run("xhr and fetch gated by xhr flag", func(t *testing.T) {
		t.Parallel()
		require.False(t, shouldInspectFetchResource(proto.NetworkResourceTypeXHR, &types.Options{}))
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeXHR, &types.Options{XhrExtraction: true}))
		require.True(t, shouldInspectFetchResource(proto.NetworkResourceTypeFetch, &types.Options{XhrExtraction: true}))
	})
}

func TestFetchRequestPatterns(t *testing.T) {
	t.Parallel()

	t.Run("default only document", func(t *testing.T) {
		t.Parallel()
		patterns := fetchRequestPatterns(&types.Options{})
		require.Len(t, patterns, 1)
		require.Equal(t, proto.NetworkResourceTypeDocument, patterns[0].ResourceType)
	})

	t.Run("js crawl adds script and stylesheet", func(t *testing.T) {
		t.Parallel()
		patterns := fetchRequestPatterns(&types.Options{ScrapeJSResponses: true})
		typesSeen := map[proto.NetworkResourceType]bool{}
		for _, p := range patterns {
			typesSeen[p.ResourceType] = true
		}
		require.True(t, typesSeen[proto.NetworkResourceTypeDocument])
		require.True(t, typesSeen[proto.NetworkResourceTypeScript])
		require.True(t, typesSeen[proto.NetworkResourceTypeStylesheet])
		require.False(t, typesSeen[proto.NetworkResourceTypeXHR])
	})

	t.Run("xhr extraction adds xhr fetch script", func(t *testing.T) {
		t.Parallel()
		patterns := fetchRequestPatterns(&types.Options{XhrExtraction: true})
		typesSeen := map[proto.NetworkResourceType]bool{}
		for _, p := range patterns {
			typesSeen[p.ResourceType] = true
		}
		require.True(t, typesSeen[proto.NetworkResourceTypeDocument])
		require.True(t, typesSeen[proto.NetworkResourceTypeScript])
		require.True(t, typesSeen[proto.NetworkResourceTypeXHR])
		require.True(t, typesSeen[proto.NetworkResourceTypeFetch])
	})
}
