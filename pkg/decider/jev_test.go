package decider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestJev(t *testing.T) {
	var got struct {
		Model     string                    `json:"model"`
		State     map[string]string         `json:"state"`
		Questions map[string]map[string]any `json:"questions"`
	}
	answers := map[Kind]string{
		Bool:   `{"type":"noul","noul":0.8}`,
		Choice: `{"type":"choice","choice":"zip","confidence":0.98}`,
		Score:  `{"type":"score","score":3,"confidence":0.6}`,
	}
	var kind Kind
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "Bearer k", r.Header.Get("Authorization"))
		require.NoError(t, json.NewDecoder(r.Body).Decode(&got))
		_, _ = w.Write([]byte(`{"model":"jev-1.14.0","answers":{"answer":` + answers[kind] + `}}`))
	}))
	defer srv.Close()
	jev := NewJev(srv.URL, "", "k", time.Second)

	kind = Choice
	a, err := jev.Decide(context.Background(), zip)
	require.NoError(t, err)
	require.Equal(t, "zip", a.Choice)
	require.Equal(t, defaultJevModel, got.Model)
	require.Equal(t, zip.Context, got.State["page"])
	require.Equal(t, "choice", got.Questions["answer"]["type"])
	require.Equal(t, map[string]any{"zip": nil, "date": nil}, got.Questions["answer"]["criteria"])

	kind = Bool
	a, err = jev.Decide(context.Background(), Question{Kind: Bool, Prompt: "login form?"})
	require.NoError(t, err)
	require.True(t, a.Bool)
	require.Equal(t, "noul", got.Questions["answer"]["type"])

	kind = Score
	a, err = jev.Decide(context.Background(), Question{Kind: Score, Prompt: "how sensitive?"})
	require.NoError(t, err)
	require.InDelta(t, 0.75, a.Score, 1e-9, "score is the level position scaled to [0, 1]")
}

func TestJevErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Header.Get("Authorization") {
		case "Bearer bad":
			http.Error(w, "invalid key", http.StatusUnauthorized)
		default:
			_, _ = w.Write([]byte(`{"answers":{"answer":{"type":"noul"}}}`))
		}
	}))
	defer srv.Close()

	_, err := NewJev(srv.URL, "", "bad", time.Second).Decide(context.Background(), zip)
	require.ErrorIs(t, err, ErrUnavailable)
	_, err = NewJev(srv.URL, "", "k", time.Second).Decide(context.Background(), Question{Kind: Bool, Prompt: "a"})
	require.Error(t, err, "an answer without its value is rejected")
}
