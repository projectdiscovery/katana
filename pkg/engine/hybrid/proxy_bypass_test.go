package hybrid

import "testing"

// Chrome bypasses the proxy for localhost/127.0.0.0/8/[::1]/link-local even
// when one is configured, so a proxy alone does not capture a crawl of a local
// target -- the exact setup used when pointing katana at an app through an
// intercepting proxy. "<-loopback>" subtracts that built-in rule.
func TestProxyBypassList(t *testing.T) {
	if got := proxyBypassList("http://127.0.0.1:8080"); got != "<-loopback>" {
		t.Errorf("proxyBypassList with a proxy = %q, want %q; without it Chrome silently skips the proxy for local targets", got, "<-loopback>")
	}
	// No proxy means nothing to bypass; sending the token anyway would be a
	// meaningless flag on every default crawl.
	if got := proxyBypassList(""); got != "" {
		t.Errorf("proxyBypassList without a proxy = %q, want empty", got)
	}
}
