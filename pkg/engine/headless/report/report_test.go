package report

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollector_BasicCollection(t *testing.T) {
	c := NewCollector("https://example.com")

	c.Collect(&output.Result{
		Request:  &navigation.Request{Method: "GET", URL: "https://example.com/"},
		Response: &navigation.Response{StatusCode: 200, Technologies: []string{"Next.js", "React"}},
	})
	c.Collect(&output.Result{
		Request:  &navigation.Request{Method: "GET", URL: "https://example.com/login"},
		Response: &navigation.Response{StatusCode: 200},
	})
	c.Collect(&output.Result{
		Request:  &navigation.Request{Method: "POST", URL: "https://example.com/api/auth/login"},
		Response: &navigation.Response{StatusCode: 200},
	})

	report := c.GenerateReport()

	assert.Equal(t, "https://example.com", report.TargetURL)
	assert.Equal(t, 3, report.TotalRequests)
	assert.Contains(t, report.Technologies, "Next.js")
	assert.Contains(t, report.Technologies, "React")
	assert.Equal(t, 3, len(report.Endpoints))
}

func TestCollector_Deduplication(t *testing.T) {
	c := NewCollector("https://example.com")

	for i := 0; i < 5; i++ {
		c.Collect(&output.Result{
			Request:  &navigation.Request{Method: "GET", URL: "https://example.com/page"},
			Response: &navigation.Response{StatusCode: 200},
		})
	}

	report := c.GenerateReport()
	assert.Equal(t, 5, report.TotalRequests)
	assert.Equal(t, 1, len(report.Endpoints))
}

func TestCollector_NilHandling(t *testing.T) {
	c := NewCollector("https://example.com")
	c.Collect(nil)
	c.Collect(&output.Result{Request: nil})
	report := c.GenerateReport()
	assert.Equal(t, 0, report.TotalRequests)
}

func TestClassifyEndpoint(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		method     string
		hasForms   bool
		expectCats []string
	}{
		{"login page", "https://example.com/login", "GET", false, []string{"auth"}},
		{"api auth", "https://example.com/api/auth/login", "POST", false, []string{"auth", "mutation", "api"}},
		{"admin panel", "https://example.com/admin/users", "GET", false, []string{"admin"}},
		{"file upload", "https://example.com/api/upload", "POST", false, []string{"file", "mutation", "api"}},
		{"search param", "https://example.com/search?q=test", "GET", false, []string{"search", "parameterized"}},
		{"IDOR user", "https://example.com/users/123", "GET", false, []string{"user_specific"}},
		{"IDOR uuid", "https://example.com/users/550e8400-e29b-41d4-a716-446655440000", "GET", false, []string{"user_specific"}},
		{"REST API", "https://example.com/api/v2/orders", "GET", false, []string{"api"}},
		{"GraphQL", "https://example.com/graphql", "POST", false, []string{"mutation", "api"}},
		{"SSRF url param", "https://example.com/proxy?url=http://internal", "GET", false, []string{"external_input", "parameterized"}},
		{"redirect param", "https://example.com/login?redirect=https://evil.com", "GET", false, []string{"auth", "external_input", "parameterized"}},
		{"DELETE", "https://example.com/api/items/5", "DELETE", false, []string{"mutation", "api"}},
		{"password reset", "https://example.com/reset-password", "GET", false, []string{"auth"}},
		{"plain page", "https://example.com/about", "GET", false, nil},
		{"dashboard", "https://example.com/dashboard", "GET", false, []string{"admin"}},
		{"with form", "https://example.com/contact", "GET", true, []string{"has_form"}},
		{"setup page", "https://example.com/setup.php", "GET", false, []string{"admin"}},
		{"phpinfo", "https://example.com/phpinfo.php", "GET", false, []string{"admin"}}, // info disclosure
		{"brute force", "https://example.com/vulnerabilities/brute/", "GET", false, []string{"auth"}},
		{"fi with page param", "https://example.com/vulnerabilities/fi/?page=include.php", "GET", false, []string{"external_input", "parameterized"}},
		{"exec page", "https://example.com/vulnerabilities/exec/?ip=127.0.0.1", "GET", false, []string{"parameterized"}},
		{"sqli with id", "https://example.com/vulnerabilities/sqli/?id=1&Submit=Submit", "GET", false, []string{"parameterized"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyEndpoint(tt.url, tt.method, nil, tt.hasForms)
			if tt.expectCats == nil {
				assert.Empty(t, got)
			} else {
				for _, cat := range tt.expectCats {
					assert.Contains(t, got, cat, "expected %q for %s %s", cat, tt.method, tt.url)
				}
			}
		})
	}
}

func TestClassifyEndpoint_FileUploadForm(t *testing.T) {
	forms := []navigation.Form{
		{Method: "POST", Enctype: "multipart/form-data", Parameters: []string{"file", "description"}},
	}
	cats := classifyEndpoint("https://example.com/submit", "POST", forms, true)
	assert.Contains(t, cats, "file")
	assert.Contains(t, cats, "mutation")
	assert.Contains(t, cats, "has_form")
}

func TestAttackSurfaceClassification(t *testing.T) {
	c := NewCollector("https://example.com")

	results := []*output.Result{
		{Request: &navigation.Request{Method: "GET", URL: "https://example.com/"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "https://example.com/login"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "POST", URL: "https://example.com/api/auth/login"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "https://example.com/admin"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "https://example.com/api/v1/users/42"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "https://example.com/search?q=test"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "POST", URL: "https://example.com/api/upload"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "https://example.com/about"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "https://example.com/vulnerabilities/sqli/?id=1"}, Response: &navigation.Response{StatusCode: 200}},
	}

	for _, r := range results {
		c.Collect(r)
	}

	report := c.GenerateReport()
	as := report.AttackSurface

	assert.Greater(t, len(as.AuthEndpoints), 0, "should have auth endpoints")
	assert.Greater(t, len(as.AdminEndpoints), 0, "should have admin endpoints")
	assert.Greater(t, len(as.SearchEndpoints), 0, "should have search endpoints")
	assert.Greater(t, len(as.FileEndpoints), 0, "should have file endpoints")
	assert.Greater(t, len(as.UserSpecific), 0, "should have user-specific endpoints")
	assert.Greater(t, len(as.APIEndpoints), 0, "should have API endpoints")
	assert.Greater(t, len(as.DataMutation), 0, "should have mutation endpoints")
	assert.Greater(t, len(as.Parameterized), 0, "should have parameterized endpoints")
	assert.Equal(t, 1, as.Summary.UniqueHosts)
}

func TestUncategorizedEndpoints(t *testing.T) {
	c := NewCollector("https://example.com")
	c.Collect(&output.Result{
		Request:  &navigation.Request{Method: "GET", URL: "https://example.com/about"},
		Response: &navigation.Response{StatusCode: 200},
	})
	c.Collect(&output.Result{
		Request:  &navigation.Request{Method: "GET", URL: "https://example.com/contact"},
		Response: &navigation.Response{StatusCode: 200},
	})

	report := c.GenerateReport()
	assert.Equal(t, 2, report.AttackSurface.Summary.Uncategorized)
	assert.Len(t, report.AttackSurface.Uncategorized, 2)
}

func TestParameterExtraction(t *testing.T) {
	c := NewCollector("https://example.com")
	c.Collect(&output.Result{
		Request:  &navigation.Request{Method: "GET", URL: "https://example.com/page?id=1&sort=name&dir=asc"},
		Response: &navigation.Response{StatusCode: 200},
	})

	report := c.GenerateReport()
	require.Len(t, report.Endpoints, 1)
	ep := report.Endpoints[0]
	assert.True(t, ep.HasParams)
	assert.Contains(t, ep.ParamNames, "id")
	assert.Contains(t, ep.ParamNames, "sort")
	assert.Contains(t, ep.ParamNames, "dir")
}

func TestSecurityHeaderAnalysis(t *testing.T) {
	c := NewCollector("https://example.com")
	c.Collect(&output.Result{
		Request: &navigation.Request{Method: "GET", URL: "https://example.com/"},
		Response: &navigation.Response{
			StatusCode: 200,
			Headers:    navigation.Headers{"Server": "Apache/2.4"},
		},
	})

	report := c.GenerateReport()
	assert.Greater(t, len(report.SecurityHeaders), 0, "should report missing security headers")

	// Check specific missing headers
	var headerNames []string
	for _, h := range report.SecurityHeaders {
		headerNames = append(headerNames, h.Header)
		assert.Equal(t, "missing", h.Status)
	}
	assert.Contains(t, headerNames, "x-frame-options")
	assert.Contains(t, headerNames, "content-security-policy")
}

func TestSecurityHeaders_CORSWildcard(t *testing.T) {
	c := NewCollector("https://example.com")
	c.Collect(&output.Result{
		Request: &navigation.Request{Method: "GET", URL: "https://example.com/api"},
		Response: &navigation.Response{
			StatusCode: 200,
			Headers: navigation.Headers{
				"Access-Control-Allow-Origin": "*",
				"X-Frame-Options":            "DENY",
				"X-Content-Type-Options":      "nosniff",
				"Content-Security-Policy":     "default-src 'self'",
				"Strict-Transport-Security":   "max-age=31536000",
			},
		},
	})

	report := c.GenerateReport()
	var corsFindings []HeaderFinding
	for _, h := range report.SecurityHeaders {
		if h.Header == "access-control-allow-origin" {
			corsFindings = append(corsFindings, h)
		}
	}
	require.Len(t, corsFindings, 1)
	assert.Equal(t, "weak", corsFindings[0].Status)
	assert.Equal(t, "*", corsFindings[0].Value)
}

func TestRenderJSON(t *testing.T) {
	c := NewCollector("https://example.com")
	c.Collect(&output.Result{
		Request:  &navigation.Request{Method: "GET", URL: "https://example.com/login"},
		Response: &navigation.Response{StatusCode: 200, Technologies: []string{"Express"}},
	})

	report := c.GenerateReport()
	var buf bytes.Buffer
	require.NoError(t, RenderJSON(&buf, report))

	var parsed map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &parsed))
	assert.Equal(t, "https://example.com", parsed["target_url"])
}

func TestRenderMarkdown(t *testing.T) {
	c := NewCollector("https://example.com")
	c.Collect(&output.Result{
		Request: &navigation.Request{Method: "GET", URL: "https://example.com/login"},
		Response: &navigation.Response{
			StatusCode:   200,
			Technologies: []string{"Express", "React"},
			Forms: []navigation.Form{
				{Method: "POST", Action: "/api/login", Parameters: []string{"email", "password"}},
			},
		},
	})
	c.Collect(&output.Result{
		Request:  &navigation.Request{Method: "POST", URL: "https://example.com/api/auth/login"},
		Response: &navigation.Response{StatusCode: 200},
	})
	c.Collect(&output.Result{
		Request:  &navigation.Request{Method: "GET", URL: "https://example.com/about"},
		Response: &navigation.Response{StatusCode: 200},
	})

	report := c.GenerateReport()
	var buf bytes.Buffer
	require.NoError(t, RenderMarkdown(&buf, report))

	md := buf.String()
	assert.Contains(t, md, "# Attack Surface Report")
	assert.Contains(t, md, "https://example.com")
	assert.Contains(t, md, "Express")
	assert.Contains(t, md, "Authentication Endpoints")
	assert.Contains(t, md, "/login")
	assert.Contains(t, md, "email, password")
	assert.Contains(t, md, "Complete Endpoint Inventory")
	assert.Contains(t, md, "/about") // even uncategorized endpoints appear in inventory
}

func TestXHREndpointCollection(t *testing.T) {
	c := NewCollector("https://example.com")

	c.Collect(&output.Result{
		Request: &navigation.Request{Method: "GET", URL: "https://example.com/dashboard"},
		Response: &navigation.Response{
			StatusCode: 200,
			XhrRequests: []navigation.Request{
				{Method: "GET", URL: "https://example.com/api/v1/user/me"},
				{Method: "GET", URL: "https://example.com/api/v1/notifications"},
			},
		},
	})

	report := c.GenerateReport()
	assert.Equal(t, 3, len(report.Endpoints))

	urls := make([]string, 0)
	for _, ep := range report.Endpoints {
		urls = append(urls, ep.URL)
	}
	assert.Contains(t, urls, "https://example.com/api/v1/user/me")
	assert.Contains(t, urls, "https://example.com/api/v1/notifications")

	for _, ep := range report.Endpoints {
		if strings.Contains(ep.URL, "/api/v1/") {
			assert.Equal(t, "xhr", ep.Source)
			assert.Contains(t, ep.Categories, "api")
		}
	}
}

func TestDVWALikeEndpoints(t *testing.T) {
	// Simulate DVWA-style endpoints to verify they classify well
	c := NewCollector("http://localhost:8089")

	dvwaResults := []*output.Result{
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/login.php"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/setup.php"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/security.php"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/phpinfo.php"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/vulnerabilities/brute/"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/vulnerabilities/exec/"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/vulnerabilities/sqli/?id=1&Submit=Submit"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/vulnerabilities/sqli_blind/?id=1&Submit=Submit"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/vulnerabilities/xss_r/?name=test"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/vulnerabilities/fi/?page=include.php"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/vulnerabilities/upload/"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/vulnerabilities/csrf/"}, Response: &navigation.Response{StatusCode: 200}},
		{Request: &navigation.Request{Method: "GET", URL: "http://localhost:8089/vulnerabilities/xss_s/"}, Response: &navigation.Response{StatusCode: 200}},
	}

	for _, r := range dvwaResults {
		c.Collect(r)
	}

	report := c.GenerateReport()
	as := report.AttackSurface

	// Auth: login.php and brute/
	assert.Greater(t, len(as.AuthEndpoints), 0, "login.php and brute should be auth")

	// Admin: setup.php, security.php
	assert.Greater(t, len(as.AdminEndpoints), 0, "setup.php should be admin")

	// File: upload/
	assert.Greater(t, len(as.FileEndpoints), 0, "upload should be file")

	// Parameterized: sqli, xss_r, fi, sqli_blind all have params
	assert.GreaterOrEqual(t, len(as.Parameterized), 4, "should have 4+ parameterized endpoints")

	// External input: fi/?page= matches the page= param
	assert.Greater(t, len(as.ExternalInputs), 0, "fi with page param should be external input")

	// Total — nothing should be completely invisible
	totalClassified := len(as.AuthEndpoints) + len(as.AdminEndpoints) + len(as.FileEndpoints) +
		len(as.SearchEndpoints) + len(as.DataMutation) + len(as.UserSpecific) +
		len(as.APIEndpoints) + len(as.ExternalInputs) + len(as.Parameterized) +
		len(as.HasForms) + len(as.Uncategorized)
	// Every endpoint should appear in at least one bucket
	// (some appear in multiple, so total >= len(endpoints))
	assert.GreaterOrEqual(t, totalClassified, len(report.Endpoints),
		"every endpoint should appear in at least one category or uncategorized")
}
