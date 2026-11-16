package common

import (
	"net/http"
	"net/url"
	"strings"
	"sync"

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

// RespondingHost is the host that sent resp, or the host of requested when the
// response has no request URL. A crawl can visit more than the seed host.
func RespondingHost(resp *http.Response, requested string) string {
	if resp != nil && resp.Request != nil && resp.Request.URL != nil {
		if host := resp.Request.URL.Hostname(); host != "" {
			return host
		}
	}
	if parsed, err := url.Parse(requested); err == nil {
		if host := parsed.Hostname(); host != "" {
			return host
		}
	}
	return requested
}

// StopForBlock reports resp when bot protection rejected it. The caller skips
// parsing: the body is a challenge page, not something to crawl.
func (s *Shared) StopForBlock(resp *http.Response, requested string) bool {
	protection := DetectBlock(resp)
	if protection == "" {
		return false
	}
	s.warnBlocked(RespondingHost(resp, requested), protection)
	return true
}

// warnBlocked tells the user once per host, so a blocked crawl does not end
// with a bare 403 and no reason. Info level, because warnings need -v.
func (s *Shared) warnBlocked(host, protection string) {
	alreadyHeadless := s.Options != nil && s.Options.Options != nil && (s.Options.Options.Headless || s.Options.Options.HeadlessHybrid)
	ReportBlock(&s.blockWarned, host, protection, alreadyHeadless)
}

// ReportBlock tells the user once per host that bot protection rejected it.
// alreadyHeadless suppresses the -headless hint, which only helps a crawl
// that is not already running a browser.
func ReportBlock(warned *sync.Map, host, protection string, alreadyHeadless bool) {
	if warned == nil {
		return
	}
	if _, seen := warned.LoadOrStore(host, struct{}{}); seen {
		return
	}
	gologger.Info().Msgf("%s is blocked by %s%s", host, protection, blockHint(protection, alreadyHeadless))
}

func blockHint(protection string, alreadyHeadless bool) string {
	if protection == cloudflareChallenge && !alreadyHeadless {
		return ", try -headless"
	}
	return ""
}
