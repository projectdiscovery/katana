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
	require.False(t, shouldAttachExtraHeaders("https://cdn.third-party.test/lib.js", nil, "https://app.example.test/"))
	require.True(t, shouldAttachExtraHeaders("https://app.example.test/api", nil, "https://app.example.test/"))
}

func TestShouldAttachExtraHeadersRequiresSameOriginAndScope(t *testing.T) {
	inScope := func(u string) bool { return u == "https://app.example.test/api" }
	require.True(t, shouldAttachExtraHeaders("https://app.example.test/api", inScope, "https://app.example.test/"))
	require.False(t, shouldAttachExtraHeaders("https://cdn.third-party.test/lib.js", inScope, "https://app.example.test/"))
	require.False(t, shouldAttachExtraHeaders("https://app.example.test/other", inScope, "https://app.example.test/"))
}

func TestShouldAttachExtraHeadersSameOriginNotJustHostname(t *testing.T) {
	cookieURL := "https://app.example.test/"
	require.True(t, shouldAttachExtraHeaders("https://app.example.test/api", nil, cookieURL))
	// Different port → different origin.
	require.False(t, shouldAttachExtraHeaders("https://app.example.test:8443/api", nil, cookieURL))
	// Different scheme → different origin.
	require.False(t, shouldAttachExtraHeaders("http://app.example.test/api", nil, cookieURL))
	// Explicit default https port still same origin.
	require.True(t, shouldAttachExtraHeaders("https://app.example.test:443/api", nil, cookieURL))
}

func TestShouldAttachExtraHeadersRejectsCleartext(t *testing.T) {
	// Non-cookie extras must never ride cleartext, even when origin matches.
	cookieHTTP := "http://app.example.test/"
	require.False(t, shouldAttachExtraHeaders("http://app.example.test/api", nil, cookieHTTP))
	allowAll := func(string) bool { return true }
	require.False(t, shouldAttachExtraHeaders("http://app.example.test/api", allowAll, cookieHTTP))
	require.True(t, shouldAttachExtraHeaders("https://app.example.test/api", nil, "https://app.example.test/"))
	require.True(t, shouldAttachExtraHeaders("wss://app.example.test/ws", nil, "wss://app.example.test/"))
	require.False(t, shouldAttachExtraHeaders("ws://app.example.test/ws", nil, "ws://app.example.test/"))
}

func TestShouldAttachExtraHeadersEmptyCookieURL(t *testing.T) {
	require.False(t, shouldAttachExtraHeaders("https://app.example.test/api", nil, ""))
	require.False(t, shouldAttachExtraHeaders("https://cdn.third-party.test/lib.js", nil, ""))
}

func TestShouldAttachExtraHeadersSameOriginEvenWhenScopePermissive(t *testing.T) {
	allowAll := func(string) bool { return true }
	cookieURL := "https://app.example.test/"
	require.True(t, shouldAttachExtraHeaders("https://app.example.test/api", allowAll, cookieURL))
	require.False(t, shouldAttachExtraHeaders("https://cdn.third-party.test/lib.js", allowAll, cookieURL))
	require.False(t, shouldAttachExtraHeaders("https://other.example.test/x", allowAll, cookieURL))
	require.False(t, shouldAttachExtraHeaders("https://app.example.test:8443/x", allowAll, cookieURL))
}

func TestSameOrigin(t *testing.T) {
	require.True(t, sameOrigin("https://a.test/", "https://a.test/path"))
	require.True(t, sameOrigin("https://a.test/", "https://a.test:443/path"))
	require.False(t, sameOrigin("https://a.test/", "https://a.test:8443/path"))
	require.False(t, sameOrigin("https://a.test/", "http://a.test/path"))
	require.False(t, sameOrigin("https://a.test/", "https://b.test/path"))
	require.False(t, sameOrigin("", "https://a.test/"))
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
