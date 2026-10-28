package browser

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetAllFormsWithShadowingFieldNames(t *testing.T) {
	skipIfNoBrowser(t)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = fmt.Fprint(w, `<!doctype html><html><body>
			<form id="search" action="/search" method="get"><input name="id"></form>
			<form action="/save" method="post"><input name="action"><input name="method"></form>
		</body></html>`)
	}))
	defer server.Close()

	launcher, err := NewLauncher(LauncherOptions{MaxBrowsers: 1, NoSandbox: true})
	require.NoError(t, err)
	defer launcher.Close()

	page, err := launcher.GetPageFromPool()
	require.NoError(t, err)
	defer launcher.PutBrowserToPool(page)
	require.NoError(t, page.Navigate(server.URL))
	require.NoError(t, page.WaitLoad())

	forms, err := page.GetAllForms()
	require.NoError(t, err)
	require.Len(t, forms, 2)
	require.Equal(t, "search", forms[0].ID)
	require.Equal(t, server.URL+"/search", forms[0].Action)
	require.Equal(t, "get", forms[0].Method)
	require.Empty(t, forms[1].ID)
	require.Equal(t, server.URL+"/save", forms[1].Action)
	require.Equal(t, "post", forms[1].Method)
}
