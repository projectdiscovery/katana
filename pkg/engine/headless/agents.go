package headless

import (
	"fmt"
	"strings"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/types"
)

// anonAgent is the -headless-agent value for a crawl without credentials.
const anonAgent = "anon"

// agent is one crawl identity. Each crawl launches its own browser with a
// fresh profile, so identities never share cookies or storage.
type agent struct {
	// label names the identity in output and logs: anon, or the username.
	label       string
	credentials string // username:password, empty for an anonymous crawl
}

// buildAgents returns the identities to crawl as. Without -headless-agent it
// is one identity with the -auto-login credentials, as before. A shared
// profile (-cdd) or browser (-cwu) holds a single session, so it keeps one.
func buildAgents(opts *types.Options) []agent {
	single := []agent{{label: anonAgent, credentials: opts.AuthCredentials}}
	if opts.AuthCredentials != "" {
		single[0].label = username(opts.AuthCredentials)
	}
	if len(opts.HeadlessAgents) == 0 {
		return single
	}
	if opts.ChromeDataDir != "" || opts.ChromeWSUrl != "" {
		gologger.Warning().Msgf("-headless-agent is ignored with -chrome-data-dir or -chrome-ws-url: a shared browser holds one session")
		return single
	}
	if header := sessionHeader(opts); header != "" {
		gologger.Warning().Msgf("-headless-agent is ignored with a %s header: every identity would send the same session", header)
		return single
	}

	agents := make([]agent, 0, len(opts.HeadlessAgents))
	seen := map[string]bool{}
	labels := map[string]int{}
	for _, value := range opts.HeadlessAgents {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if seen[value] {
			// Two crawls with one account can log each other out on sites
			// that allow a single session.
			gologger.Warning().Msgf("-headless-agent %q is given more than once; it is crawled once", agentLabel(value))
			continue
		}
		seen[value] = true
		a := agent{label: agentLabel(value)}
		// One username with other credentials is another identity: number
		// it, so its results and URL dedupe stay apart without the password.
		labels[a.label]++
		if n := labels[a.label]; n > 1 {
			a.label = fmt.Sprintf("%s#%d", a.label, n)
		}
		if value != anonAgent {
			a.credentials = value
		}
		agents = append(agents, a)
	}
	if len(agents) == 0 {
		return single
	}
	return agents
}

// sessionHeader returns the name of a -H header that carries a session
// (Cookie or Authorization), or "" if there is none. Every identity sends the
// -H headers, so such a header would make all identities one session.
func sessionHeader(opts *types.Options) string {
	for name := range opts.ParseCustomHeaders() {
		if strings.EqualFold(name, "Cookie") || strings.EqualFold(name, "Authorization") {
			return name
		}
	}
	return ""
}

func agentLabel(value string) string {
	if value == anonAgent {
		return anonAgent
	}
	return username(value)
}

func username(credentials string) string {
	user, _ := splitCredentials(credentials)
	return user
}

func splitCredentials(creds string) (user, pass string) {
	parts := strings.SplitN(creds, ":", 2)
	user = parts[0]
	if len(parts) > 1 {
		pass = parts[1]
	}
	return user, pass
}

// dedupScope returns the prefix that keeps the URL dedupe of one identity
// apart from the others: a page another identity already reported can show
// different content to this one.
func (a agent) dedupScope(agents int) string {
	if agents <= 1 {
		return ""
	}
	return fmt.Sprintf("agent:%s|", a.label)
}
