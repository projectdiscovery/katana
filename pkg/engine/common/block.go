package common

import (
	"net/http"
	"strings"

	"github.com/projectdiscovery/gologger"
)

const cloudflareChallenge = "Cloudflare challenge"

// DetectBlock returns the bot protection that rejected resp, or "" if none.
// These reject the client itself rather than its request rate, so backing off
// does not help and the response has nothing to crawl.
func DetectBlock(resp *http.Response) string {
	if resp == nil || resp.StatusCode != http.StatusForbidden {
		return ""
	}
	h := resp.Header
	switch {
	case strings.EqualFold(h.Get("Cf-Mitigated"), "challenge"):
		return cloudflareChallenge
	case h.Get("X-Datadome") != "":
		return "DataDome"
	case h.Get("X-Px-Blocked") != "":
		return "PerimeterX"
	case strings.HasPrefix(h.Get("Server"), "AkamaiGHost"):
		return "Akamai"
	}
	return ""
}

// warnBlocked tells the user once per host, so a blocked crawl does not end
// with a bare 403 and no reason. Info level, because warnings need -v.
func (s *Shared) warnBlocked(host, protection string) {
	if _, warned := s.blockWarned.LoadOrStore(host, struct{}{}); warned {
		return
	}
	// A headless browser solves the Cloudflare JS challenge; the others block
	// browsers too.
	hint := ""
	if protection == cloudflareChallenge {
		hint = ", try -headless"
	}
	gologger.Info().Msgf("%s is blocked by %s%s", host, protection, hint)
}
