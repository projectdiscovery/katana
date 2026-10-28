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
		if value != anonAgent {
			a.credentials = value
		}
		agents = append(agents, a)
	}
	if len(agents) == 0 {
		return single
	}
	uniqueLabels(agents)
	return agents
}

// uniqueLabels makes the label of each agent unique, as each label is also
// the dedupe scope of the agent. The first agent with a username keeps it as
// its label, and so does a username that is already numbered, such as bob#2.
// Another agent with the same username (other credentials) gets the next free
// number: bob#2, bob#3 and so on. The label anon is only for the anonymous
// agent, so an account named anon is numbered too.
func uniqueLabels(agents []agent) {
	taken := map[string]bool{anonAgent: true}
	numbered := make([]bool, len(agents))
	for i, a := range agents {
		if a.credentials == "" {
			continue
		}
		if taken[a.label] {
			numbered[i] = true
			continue
		}
		taken[a.label] = true
	}
	for i := range agents {
		if !numbered[i] {
			continue
		}
		for n := 2; ; n++ {
			label := fmt.Sprintf("%s#%d", agents[i].label, n)
			if !taken[label] {
				taken[label] = true
				agents[i].label = label
				break
			}
		}
	}
}

// sessionHeaders are the -H headers that katana treats as a session: the
// standard ones and the API key headers that the README documents for
// authenticated crawling. Other header names are not treated as a session.
var sessionHeaders = []string{"Cookie", "Authorization", "X-API-Key", "X-Auth-Token", "X-Access-Token"}

// sessionHeader returns the name of a -H header that carries a session (one
// of sessionHeaders), or "" if there is none. Every identity sends the -H
// headers, so such a header would make all identities one session.
func sessionHeader(opts *types.Options) string {
	for name := range opts.ParseCustomHeaders() {
		for _, session := range sessionHeaders {
			if strings.EqualFold(strings.TrimSpace(name), session) {
				return name
			}
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
