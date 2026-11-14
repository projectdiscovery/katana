package common

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDetectBlock(t *testing.T) {
	tests := []struct {
		name   string
		status int
		header http.Header
		want   string
	}{
		{"cloudflare challenge", http.StatusForbidden, http.Header{"Cf-Mitigated": {"challenge"}, "Server": {"cloudflare"}}, cloudflareChallenge},
		{"datadome", http.StatusForbidden, http.Header{"X-Datadome": {"protected"}, "Server": {"cloudflare"}}, "DataDome"},
		{"perimeterx", http.StatusForbidden, http.Header{"X-Px-Blocked": {"1"}, "Server": {"CloudFront"}}, "PerimeterX"},
		{"akamai", http.StatusForbidden, http.Header{"Server": {"AkamaiGHost"}}, "Akamai"},
		{"plain 403 behind cloudflare", http.StatusForbidden, http.Header{"Server": {"cloudflare"}}, ""},
		{"datadome header on a served page", http.StatusOK, http.Header{"X-Datadome": {"protected"}}, ""},
		{"akamai served page", http.StatusOK, http.Header{"Server": {"AkamaiGHost"}}, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, DetectBlock(&http.Response{StatusCode: tt.status, Header: tt.header}))
		})
	}
	require.Empty(t, DetectBlock(nil))
}

func TestWarnBlockedOncePerHost(t *testing.T) {
	shared := &Shared{}
	shared.warnBlocked("a.com", "Akamai")
	shared.warnBlocked("a.com", "Akamai")
	shared.warnBlocked("b.com", "Akamai")

	hosts := 0
	shared.blockWarned.Range(func(_, _ any) bool { hosts++; return true })
	require.Equal(t, 2, hosts)
}
