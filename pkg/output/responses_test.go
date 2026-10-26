package output

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils/extensions"
	"github.com/stretchr/testify/require"
)

func TestResponseFileNamePerAgent(t *testing.T) {
	dir := t.TempDir()
	const u = "https://example.com/account"
	single := getResponseFileName(dir, "example.com", u, "")
	require.Equal(t, filepath.Join(dir, "example.com", getResponseHash(u)+".txt"), single, "a crawl as one identity keeps its file name")

	anon := getResponseFileName(dir, "example.com", u, "anon")
	bob := getResponseFileName(dir, "example.com", u, "bob")
	require.NotEqual(t, anon, bob, "two identities must not share a stored response")
	require.NotEqual(t, single, anon)
}

func TestStoreResponsePerAgent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "responses")
	w, err := New(Options{StoreResponse: true, StoreResponseDir: dir, JSON: true, ExtensionValidator: extensions.NewValidator(nil, nil, false)})
	require.NoError(t, err)

	const u = "https://example.com/account"
	write := func(agent, body string) string {
		req, _ := http.NewRequest(http.MethodGet, u, nil)
		res := &Result{
			Request:  &navigation.Request{Method: http.MethodGet, URL: u, Agent: agent},
			Response: &navigation.Response{StatusCode: 200, Body: body, Resp: &http.Response{StatusCode: 200, Status: "200 OK", Request: req}},
		}
		require.NoError(t, w.Write(res))
		return res.Response.StoredResponsePath
	}
	anon := write("anon", "anon page")
	bob := write("bob", "bob page")
	require.NoError(t, w.Close())
	require.NotEqual(t, anon, bob)

	for _, path := range []string{anon, bob} {
		_, err := os.Stat(path)
		require.NoError(t, err, "each identity must keep its stored response")
	}

	index, err := os.ReadFile(filepath.Join(dir, indexFile))
	require.NoError(t, err)
	for _, path := range []string{anon, bob} {
		require.True(t, strings.Contains(string(index), filepath.Base(path)), "the index must name %s", path)
	}
}
