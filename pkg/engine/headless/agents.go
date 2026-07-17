package headless

import (
	"fmt"
	"os"
	"strings"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/cartography"
	"github.com/projectdiscovery/katana/pkg/types"
)

func buildAgentPool(opts *types.Options) *cartography.AgentPool {
	if opts == nil {
		return cartography.NewAgentPool()
	}
	if opts.ChromeDataDir != "" || opts.ChromeWSUrl != "" {
		return cartography.NewAgentPool(&cartography.Agent{
			ID:      "default",
			Role:    "anon",
			DataDir: opts.ChromeDataDir,
		})
	}
	n := opts.HeadlessAgents
	if n <= 0 {
		n = 1
	}
	agents := make([]*cartography.Agent, 0, n)
	for i := 0; i < n; i++ {
		a := &cartography.Agent{
			ID:   fmt.Sprintf("agent-%d", i),
			Role: "anon",
		}
		if opts.AuthCredentials != "" && i == 0 {
			a.Credentials = opts.AuthCredentials
			a.Role = "auth"
		}
		agents = append(agents, a)
	}
	pool := cartography.NewAgentPool(agents...)
	if cartography.DetectSingleLoginConflict(pool.All()) {
		gologger.Warning().Msg("headless agents share credentials; prefer distinct accounts")
	}
	return pool
}

func ensureAgentDataDir(agent *cartography.Agent) (cleanup func(), err error) {
	cleanup = func() {}
	if agent == nil || agent.DataDir != "" {
		return cleanup, nil
	}
	dir, err := os.MkdirTemp("", fmt.Sprintf("katana-agent-%s-*", agent.ID))
	if err != nil {
		return cleanup, err
	}
	agent.DataDir = dir
	return func() { _ = os.RemoveAll(dir) }, nil
}

func splitCredentials(creds string) (user, pass string) {
	parts := strings.SplitN(creds, ":", 2)
	user = parts[0]
	if len(parts) > 1 {
		pass = parts[1]
	}
	return user, pass
}
