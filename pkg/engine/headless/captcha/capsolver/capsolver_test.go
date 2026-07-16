package capsolver

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/captcha"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSolve(t *testing.T) {
	var pollCount atomic.Int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)

		switch r.URL.Path {
		case "/createTask":
			task, ok := body["task"].(map[string]any)
			assert.True(t, ok)
			assert.Equal(t, "ReCaptchaV2TaskProxyLess", task["type"])
			assert.Equal(t, "https://example.com", task["websiteURL"])
			assert.Equal(t, "test-sitekey", task["websiteKey"])

			_ = json.NewEncoder(w).Encode(map[string]any{
				"errorId": 0,
				"taskId":  "task-123",
			})

		case "/getTaskResult":
			assert.Equal(t, "task-123", body["taskId"])

			count := pollCount.Add(1)
			if count < 2 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"errorId": 0,
					"status":  "processing",
				})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errorId": 0,
				"status":  "ready",
				"solution": map[string]any{
					"gRecaptchaResponse": "solved-token-abc",
				},
			})
		}
	}))
	defer server.Close()

	origURL := baseURL
	SetBaseURL(server.URL)
	defer SetBaseURL(origURL)

	cs := New("test-api-key")
	solution, err := cs.Solve(context.Background(), &captcha.Info{
		Provider: captcha.ProviderRecaptchaV2,
		SiteKey:  "test-sitekey",
		PageURL:  "https://example.com",
	})

	require.NoError(t, err)
	require.NotNil(t, solution)
	assert.Equal(t, "solved-token-abc", solution.Token)
	assert.Equal(t, captcha.ProviderRecaptchaV2, solution.Provider)
	assert.GreaterOrEqual(t, int(pollCount.Load()), 2)
}

func TestCreateTaskError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"errorId":          1,
			"errorCode":        "ERROR_KEY_DENIED",
			"errorDescription": "Account not found or blocked",
		})
	}))
	defer server.Close()

	origURL := baseURL
	SetBaseURL(server.URL)
	defer SetBaseURL(origURL)

	cs := New("bad-key")
	_, err := cs.Solve(context.Background(), &captcha.Info{
		Provider: captcha.ProviderRecaptchaV2,
		SiteKey:  "key",
		PageURL:  "https://example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ERROR_KEY_DENIED")
}

func TestTurnstileToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/createTask":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errorId": 0,
				"taskId":  "task-turnstile",
			})
		case "/getTaskResult":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"errorId": 0,
				"status":  "ready",
				"solution": map[string]any{
					"token": "turnstile-token-xyz",
				},
			})
		}
	}))
	defer server.Close()

	origURL := baseURL
	SetBaseURL(server.URL)
	defer SetBaseURL(origURL)

	cs := New("key")
	solution, err := cs.Solve(context.Background(), &captcha.Info{
		Provider: captcha.ProviderTurnstile,
		SiteKey:  "cf-key",
		PageURL:  "https://example.com",
	})
	require.NoError(t, err)
	assert.Equal(t, "turnstile-token-xyz", solution.Token)
	assert.Equal(t, captcha.ProviderTurnstile, solution.Provider)
}

func TestExtractToken(t *testing.T) {
	tests := []struct {
		name     string
		solution map[string]any
		provider captcha.Provider
		want     string
		wantErr  bool
	}{
		{
			name:     "recaptcha v2",
			solution: map[string]any{"gRecaptchaResponse": "token-a"},
			provider: captcha.ProviderRecaptchaV2,
			want:     "token-a",
		},
		{
			name:     "turnstile",
			solution: map[string]any{"token": "token-b"},
			provider: captcha.ProviderTurnstile,
			want:     "token-b",
		},
		{
			name:     "missing token",
			solution: map[string]any{},
			provider: captcha.ProviderRecaptchaV2,
			wantErr:  true,
		},
		{
			name:     "empty token",
			solution: map[string]any{"gRecaptchaResponse": ""},
			provider: captcha.ProviderHCaptcha,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sol, err := extractToken(tt.solution, tt.provider)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, sol.Token)
		})
	}
}

func TestUnsupportedProvider(t *testing.T) {
	cs := New("key")
	_, err := cs.Solve(context.Background(), &captcha.Info{
		Provider: "unknown",
		SiteKey:  "key",
		PageURL:  "https://example.com",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}
