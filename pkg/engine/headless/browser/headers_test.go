package browser

import (
	"testing"

	"github.com/go-rod/rod/lib/proto"
	"github.com/stretchr/testify/require"
	"github.com/ysmood/gson"
)

func TestParseCookieHeaderScopesToRootURL(t *testing.T) {
	cookies := parseCookieHeader("session=abc=123; theme=dark; invalid", "https://app.example.test/dashboard")
	require.Len(t, cookies, 2)
	require.Equal(t, "session", cookies[0].Name)
	require.Equal(t, "abc=123", cookies[0].Value)
	require.Equal(t, "https://app.example.test/dashboard", cookies[0].URL)
	require.Equal(t, "/", cookies[0].Path)
	require.Equal(t, "theme", cookies[1].Name)
}

func TestSplitCustomHeadersKeepsCookieScopedAndExtraSeparate(t *testing.T) {
	cookies, extra := splitCustomHeaders(map[string]string{
		"Cookie":        "session=ok",
		"Authorization": "Bearer secret",
		"X-Trace":       "1",
	}, "https://app.example.test/")
	require.Len(t, cookies, 1)
	require.Equal(t, "session", cookies[0].Name)
	require.Equal(t, "https://app.example.test/", cookies[0].URL)
	require.Equal(t, map[string]string{
		"Authorization": "Bearer secret",
		"X-Trace":       "1",
	}, extra)
	// Extra headers must not be applied page-wide; attachment is scope-gated.
	require.False(t, shouldAttachExtraHeaders("https://cdn.third-party.test/lib.js", nil, "https://app.example.test/"))
	require.True(t, shouldAttachExtraHeaders("https://app.example.test/api", nil, "https://app.example.test/"))
}

func TestShouldAttachExtraHeadersUsesScopeValidator(t *testing.T) {
	inScope := func(u string) bool { return u == "https://app.example.test/api" }
	require.True(t, shouldAttachExtraHeaders("https://app.example.test/api", inScope, "https://app.example.test/"))
	require.False(t, shouldAttachExtraHeaders("https://cdn.third-party.test/lib.js", inScope, "https://app.example.test/"))
}

func TestShouldAttachExtraHeadersFallsBackToCookieHost(t *testing.T) {
	cookieURL := "https://app.example.test/dashboard"
	require.True(t, shouldAttachExtraHeaders("https://app.example.test/api", nil, cookieURL))
	require.False(t, shouldAttachExtraHeaders("https://cdn.third-party.test/lib.js", nil, cookieURL))
	require.False(t, shouldAttachExtraHeaders("https://cdn.third-party.test/lib.js", nil, ""))
}

func TestMergeRequestHeadersOverlaysExtraWithoutDroppingOthers(t *testing.T) {
	existing := proto.NetworkHeaders{
		"Accept":        gson.New("text/html"),
		"Authorization": gson.New("stale"),
	}
	extra := map[string]string{"Authorization": "Bearer secret", "X-Trace": "1"}
	merged := mergeRequestHeaders(existing, extra)
	require.Len(t, merged, 3)

	got := map[string]string{}
	for _, h := range merged {
		got[h.Name] = h.Value
	}
	require.Equal(t, "Bearer secret", got["Authorization"])
	require.Equal(t, "1", got["X-Trace"])
	require.Equal(t, "text/html", got["Accept"])
}

func TestMergeRequestHeadersNilWhenNoExtra(t *testing.T) {
	require.Nil(t, mergeRequestHeaders(proto.NetworkHeaders{"Accept": gson.New("*/*")}, nil))
}

func TestIsFetchRequestStage(t *testing.T) {
	require.True(t, isFetchRequestStage(&proto.FetchRequestPaused{}))
	status := 200
	require.False(t, isFetchRequestStage(&proto.FetchRequestPaused{ResponseStatusCode: &status}))
	require.False(t, isFetchRequestStage(&proto.FetchRequestPaused{ResponseErrorReason: proto.NetworkErrorReasonFailed}))
	require.False(t, isFetchRequestStage(nil))
}
