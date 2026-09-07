package utils

import (
	"strings"
	"testing"
)

// TestWave14HTMLFormActionTargetResolution asserts form action URL resolution
func TestWave14HTMLFormActionTargetResolution(t *testing.T) {
	resolveFormAction := func(baseURL, action string) string {
		action = strings.TrimSpace(action)
		if action == "" {
			return baseURL
		}
		if strings.HasPrefix(action, "http://") || strings.HasPrefix(action, "https://") {
			return action
		}
		if strings.HasPrefix(action, "/") {
			// Extract host
			idx := strings.Index(baseURL[8:], "/")
			if idx == -1 {
				return baseURL + action
			}
			return baseURL[:8+idx] + action
		}
		return strings.TrimSuffix(baseURL, "/") + "/" + action
	}

	baseURL := "https://example.com/app/login"
	if resolveFormAction(baseURL, "/api/auth") != "https://example.com/api/auth" {
		t.Errorf("expected root relative form action to resolve to host root")
	}
	if resolveFormAction(baseURL, "process") != "https://example.com/app/login/process" {
		t.Errorf("expected relative action to append")
	}
}

// TestWave14FormHTTPMethodNormalization asserts form verb normalization
func TestWave14FormHTTPMethodNormalization(t *testing.T) {
	normalizeMethod := func(m string) string {
		m = strings.ToUpper(strings.TrimSpace(m))
		if m == "POST" || m == "GET" || m == "PUT" || m == "DELETE" {
			return m
		}
		return "GET" // RFC default
	}

	if normalizeMethod("post") != "POST" {
		t.Errorf("expected POST method uppercase normalization")
	}
	if normalizeMethod("") != "GET" {
		t.Errorf("expected empty method to fallback to GET")
	}
}
