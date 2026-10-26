package headless

import (
	"encoding/json"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/stretchr/testify/require"
)

func TestBuildAgents(t *testing.T) {
	t.Parallel()

	require.Equal(t, []agent{{label: "anon"}}, buildAgents(&types.Options{}))
	require.Equal(t, []agent{{label: "bob", credentials: "bob:pw"}}, buildAgents(&types.Options{AuthCredentials: "bob:pw"}))

	got := buildAgents(&types.Options{
		AuthCredentials: "ignored:pw",
		HeadlessAgents:  []string{"anon", "alice:a:b", "bob:pw", "bob:pw", " "},
	})
	require.Equal(t, []agent{
		{label: "anon"},
		{label: "alice", credentials: "alice:a:b"},
		{label: "bob", credentials: "bob:pw"},
	}, got, "each identity uses its own credentials, duplicates are dropped")

	shared := buildAgents(&types.Options{ChromeWSUrl: "ws://127.0.0.1:9222", HeadlessAgents: []string{"anon", "bob:pw"}})
	require.Len(t, shared, 1)
}

func TestBuildAgentsSameUserOtherPassword(t *testing.T) {
	t.Parallel()
	got := buildAgents(&types.Options{HeadlessAgents: []string{"bob:old", "bob:new", "anon", "bob:third"}})
	require.Equal(t, []agent{
		{label: "bob", credentials: "bob:old"},
		{label: "bob#2", credentials: "bob:new"},
		{label: "anon"},
		{label: "bob#3", credentials: "bob:third"},
	}, got)
	scopes := map[string]bool{}
	for _, a := range got {
		scope := a.dedupScope(len(got))
		require.NotContains(t, scope, "old")
		require.NotContains(t, scope, "new")
		scopes[scope] = true
	}
	require.Len(t, scopes, len(got), "each identity needs its own dedupe scope")
}

func TestBuildAgentsSessionHeader(t *testing.T) {
	t.Parallel()
	for _, header := range []string{"Cookie: sid=1", "cookie:sid=1", "Authorization: Bearer x", "X-API-KEY: key", "x-auth-token: t", "X-Access-Token: t"} {
		got := buildAgents(&types.Options{CustomHeaders: []string{header}, HeadlessAgents: []string{"anon", "bob:pw"}})
		require.Equal(t, []agent{{label: "anon"}}, got, header)
	}
	got := buildAgents(&types.Options{CustomHeaders: []string{"X-Test: 1"}, HeadlessAgents: []string{"anon", "bob:pw"}})
	require.Len(t, got, 2, "other headers keep every identity")
}

func TestBuildAgentsUniqueLabels(t *testing.T) {
	t.Parallel()
	labels := func(values ...string) []string {
		var out []string
		for _, a := range buildAgents(&types.Options{HeadlessAgents: values}) {
			out = append(out, a.label)
		}
		return out
	}
	// anon is only for the anonymous agent, whatever the order.
	require.Equal(t, []string{"anon#2", "anon"}, labels("anon:password", "anon"))
	require.Equal(t, []string{"anon", "anon#2"}, labels("anon", "anon:password"))
	require.Equal(t, []string{"anon#2", "bob"}, labels("anon:password", "bob:pw"))
	// A literal numbered username keeps its label; generated numbers skip it.
	require.Equal(t, []string{"bob", "bob#3", "bob#2"}, labels("bob:old", "bob:new", "bob#2:password"))
	require.Equal(t, []string{"bob#2", "bob", "bob#3"}, labels("bob#2:password", "bob:old", "bob:new"))
	for _, values := range [][]string{
		{"bob:a", "bob:b", "bob#2:c", "bob#3:d", "bob:e"},
		{"anon:a", "anon#2:b", "anon", "anon:c"},
	} {
		got := labels(values...)
		unique := map[string]bool{}
		for _, l := range got {
			unique[l] = true
		}
		require.Len(t, unique, len(got), "labels %v must be unique", got)
	}
}

func TestAgentDedupScope(t *testing.T) {
	t.Parallel()
	a := agent{label: "bob"}
	require.Empty(t, a.dedupScope(1))
	require.NotEqual(t, a.dedupScope(2), agent{label: "anon"}.dedupScope(2))
}

func TestSplitCredentials(t *testing.T) {
	t.Parallel()
	user, pass := splitCredentials("bob:p:w")
	require.Equal(t, "bob", user)
	require.Equal(t, "p:w", pass)
}

func TestLabelAgent(t *testing.T) {
	t.Run("request_only", func(t *testing.T) {
		rr := &output.Result{Request: &navigation.Request{Method: "GET", URL: "https://example.com/a"}}
		labelAgent(rr, "bob")
		raw, err := json.Marshal(rr)
		require.NoError(t, err)
		var got struct {
			Request  map[string]any `json:"request"`
			Response map[string]any `json:"response"`
		}
		require.NoError(t, json.Unmarshal(raw, &got))
		require.Equal(t, "bob", got.Request["agent"])
		require.Nil(t, got.Response, "a request-only result must not gain a response")
	})

	t.Run("with_response", func(t *testing.T) {
		rr := &output.Result{
			Request:  &navigation.Request{Method: "GET", URL: "https://example.com/a"},
			Response: &navigation.Response{StatusCode: 200},
		}
		labelAgent(rr, "anon")
		require.Equal(t, "anon", rr.Request.Agent)
		require.Equal(t, "anon", rr.Response.Extra["agent"])
	})

	t.Run("nil_safe", func(t *testing.T) {
		labelAgent(nil, "anon")
		labelAgent(&output.Result{}, "anon")
	})
}

func TestRequestAgentOmittedWhenUnset(t *testing.T) {
	raw, err := json.Marshal(&navigation.Request{Method: "GET", URL: "https://example.com/a"})
	require.NoError(t, err)
	require.NotContains(t, string(raw), "agent", "a single-identity crawl must keep its output unchanged")
}
