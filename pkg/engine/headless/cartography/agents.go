package cartography

import (
	"fmt"
	"sync"
)

// Agent is a crawl identity with its own cookie/storage jar.
// Unlike hybrid -c browser workers (throughput), agents model distinct users/roles.
type Agent struct {
	ID          string
	Role        string // e.g. anon, user, admin
	Credentials string // optional username:password or recorded-flow ref
	DataDir     string // chrome user-data-dir for jar isolation
}

// AgentPool holds named identities for multi-agent cartography.
type AgentPool struct {
	mu     sync.Mutex
	agents []*Agent
}

// NewAgentPool creates a pool. If agents is empty, a single anon agent is used.
func NewAgentPool(agents ...*Agent) *AgentPool {
	p := &AgentPool{}
	if len(agents) == 0 {
		p.agents = []*Agent{{ID: "anon", Role: "anon"}}
		return p
	}
	for i, a := range agents {
		if a == nil {
			continue
		}
		cp := *a
		if cp.ID == "" {
			cp.ID = fmt.Sprintf("agent-%d", i)
		}
		if cp.Role == "" {
			cp.Role = "anon"
		}
		p.agents = append(p.agents, &cp)
	}
	if len(p.agents) == 0 {
		p.agents = []*Agent{{ID: "anon", Role: "anon"}}
	}
	return p
}

// All returns a snapshot of agents.
func (p *AgentPool) All() []*Agent {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]*Agent, len(p.agents))
	copy(out, p.agents)
	return out
}

// Len returns the number of agents.
func (p *AgentPool) Len() int {
	if p == nil {
		return 0
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.agents)
}

// Get returns an agent by ID.
func (p *AgentPool) Get(id string) (*Agent, bool) {
	for _, a := range p.All() {
		if a.ID == id {
			return a, true
		}
	}
	return nil, false
}

// DetectSingleLoginConflict reports whether two authenticated agents share credentials.
// Burp uses this signal to avoid concurrent logins for the same account.
func DetectSingleLoginConflict(agents []*Agent) bool {
	seen := map[string]struct{}{}
	for _, a := range agents {
		if a == nil || a.Credentials == "" {
			continue
		}
		if _, ok := seen[a.Credentials]; ok {
			return true
		}
		seen[a.Credentials] = struct{}{}
	}
	return false
}
