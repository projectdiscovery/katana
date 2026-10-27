package common

import (
	"net/http"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
	"github.com/stretchr/testify/require"
)

func TestSurfaceScore(t *testing.T) {
	tests := []struct {
		name string
		req  navigation.Request
		want int
	}{
		{"plain page", navigation.Request{Method: http.MethodGet, URL: "https://example.com/about"}, 0},
		{"keyword segment", navigation.Request{Method: http.MethodGet, URL: "https://example.com/admin/"}, 2},
		{"keyword inside a file name", navigation.Request{Method: http.MethodGet, URL: "https://example.com/user-login.php"}, 2},
		{"keyword counted once", navigation.Request{Method: http.MethodGet, URL: "https://example.com/api/users"}, 2},
		{"keyword as substring only", navigation.Request{Method: http.MethodGet, URL: "https://example.com/apiary"}, 0},
		{"keyword in host is ignored", navigation.Request{Method: http.MethodGet, URL: "https://api.example.com/about"}, 0},
		{"query parameters", navigation.Request{Method: http.MethodGet, URL: "https://example.com/item?id=1"}, 1},
		{"post body", navigation.Request{Method: http.MethodPost, URL: "https://example.com/contact", Body: "a=b"}, 1},
		{"everything", navigation.Request{Method: http.MethodPost, URL: "https://example.com/upload?dir=x"}, 4},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, surfaceScore(&tt.req))
		})
	}
}

func TestEnqueuePriorityStrategy(t *testing.T) {
	enqueue := func(strategy string) []string {
		shared, _ := newTestShared(5, func(o *types.Options) { o.Strategy = strategy })
		q, err := queue.New(strategy, 1)
		require.NoError(t, err)
		shared.Enqueue(q,
			&navigation.Request{Method: http.MethodGet, URL: "https://example.com/about", Depth: 1},
			&navigation.Request{Method: http.MethodGet, URL: "https://example.com/blog/post", Depth: 2},
			&navigation.Request{Method: http.MethodGet, URL: "https://example.com/blog/admin/", Depth: 2},
			&navigation.Request{Method: http.MethodGet, URL: "https://example.com/contact", Depth: 1},
		)
		var urls []string
		for item := range q.Pop() {
			urls = append(urls, item.(*navigation.Request).URL)
			if len(urls) == 4 {
				break
			}
		}
		return urls
	}

	got := enqueue("priority")
	require.Equal(t, "https://example.com/blog/admin/", got[0], "a deeper admin link goes first")
	require.Equal(t, "https://example.com/blog/post", got[3], "the deepest plain link goes last")

	got = enqueue("breadth-first")
	require.ElementsMatch(t, []string{"https://example.com/about", "https://example.com/contact"}, got[:2], "breadth-first still orders by depth only")
}
