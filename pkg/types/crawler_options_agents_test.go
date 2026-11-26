package types

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// A headless identity with credentials must get the classifier, otherwise the
// crawler never runs the login steps for it.
func TestNewCrawlerOptionsHeadlessAgentEnablesClassifier(t *testing.T) {
	opts := &Options{
		MaxDepth:       1,
		BodyReadSize:   1024,
		Strategy:       "depth-first",
		FieldScope:     "rdn",
		HeadlessAgents: []string{"alice:pw"},
	}
	co, err := NewCrawlerOptions(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = co.Close() })

	require.NotNil(t, co.DitClassifier, "a credentialed -headless-agent needs the classifier to log in")
}

// Anonymous identities have nothing to log in with, so they do not need it.
func TestNewCrawlerOptionsAnonymousAgentSkipsClassifier(t *testing.T) {
	opts := &Options{
		MaxDepth:       1,
		BodyReadSize:   1024,
		Strategy:       "depth-first",
		FieldScope:     "rdn",
		HeadlessAgents: []string{"anon"},
	}
	co, err := NewCrawlerOptions(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = co.Close() })

	require.Nil(t, co.DitClassifier)
}
