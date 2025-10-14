package filters

import (
	"net/url"
	"testing"
)

func TestParseDepthFilter(t *testing.T) {
	tests := []struct {
		name        string
		filter      string
		expectError bool
		expected    DepthFilter
	}{
		{
			name:        "Valid greater than or equal",
			filter:      ">=3",
			expectError: false,
			expected:    DepthFilter{Operator: ">=", Value: 3, MaxValue: 0},
		},
		{
			name:        "Valid equal",
			filter:      "==2",
			expectError: false,
			expected:    DepthFilter{Operator: "==", Value: 2, MaxValue: 0},
		},
		{
			name:        "Valid range",
			filter:      "2-5",
			expectError: false,
			expected:    DepthFilter{Operator: "range", Value: 2, MaxValue: 5},
		},
		{
			name:        "Invalid operator",
			filter:      "!=3",
			expectError: true,
		},
		{
			name:        "Invalid format",
			filter:      "3",
			expectError: true,
		},
		{
			name:        "Negative value",
			filter:      ">=-1",
			expectError: true,
		},
		{
			name:        "Invalid range",
			filter:      "5-2",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := parseDepthFilter(tt.filter)
			
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error for filter '%s', but got none", tt.filter)
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error for filter '%s': %v", tt.filter, err)
				return
			}
			
			if result.Operator != tt.expected.Operator {
				t.Errorf("Expected operator '%s', got '%s'", tt.expected.Operator, result.Operator)
			}
			
			if result.Value != tt.expected.Value {
				t.Errorf("Expected value %d, got %d", tt.expected.Value, result.Value)
			}
			
			if result.MaxValue != tt.expected.MaxValue {
				t.Errorf("Expected max value %d, got %d", tt.expected.MaxValue, result.MaxValue)
			}
		})
	}
}

func TestCountPathSegments(t *testing.T) {
	tests := []struct {
		path     string
		expected int
	}{
		{"/", 0},
		{"", 0},
		{"/api", 1},
		{"/api/v1", 2},
		{"/api/v1/users", 3},
		{"/api/v1/users/", 3}, // trailing slash ignored
		{"/api//v1", 2},       // empty segments ignored
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := countPathSegments(tt.path)
			if result != tt.expected {
				t.Errorf("For path '%s', expected %d, got %d", tt.path, tt.expected, result)
			}
		})
	}
}

func TestCountQueryParams(t *testing.T) {
	tests := []struct {
		query    string
		expected int
	}{
		{"", 0},
		{"user=admin", 1},
		{"user=admin&pass=secret", 2},
		{"user=admin&pass=secret&role=user", 3},
		{"user=admin&empty&pass=secret", 2}, // empty params ignored
		{"=value", 0},                       // invalid param ignored
	}

	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			result := countQueryParams(tt.query)
			if result != tt.expected {
				t.Errorf("For query '%s', expected %d, got %d", tt.query, tt.expected, result)
			}
		})
	}
}

func TestCountSubdomainLevels(t *testing.T) {
	tests := []struct {
		hostname string
		expected int
	}{
		{"example.com", 0},
		{"api.example.com", 1},
		{"api.v1.example.com", 2},
		{"cdn.assets.api.v1.example.com", 4},
		{"localhost", 0},
		{"example.com:8080", 0}, // port ignored
	}

	for _, tt := range tests {
		t.Run(tt.hostname, func(t *testing.T) {
			result := countSubdomainLevels(tt.hostname)
			if result != tt.expected {
				t.Errorf("For hostname '%s', expected %d, got %d", tt.hostname, tt.expected, result)
			}
		})
	}
}

func TestDepthFilterValidator(t *testing.T) {
	validator, err := NewDepthFilterValidator(
		[]string{">=2"},
		[]string{"<=3"},
		[]string{"==1"},
		false, // AND logic
	)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "Valid URL matching all filters",
			url:      "https://api.example.com/v1/users?id=1&format=json&sort=name",
			expected: true, // path=3, query=3, subdomain=1
		},
		{
			name:     "URL with insufficient path depth",
			url:      "https://api.example.com/users",
			expected: false, // path=1 (fails >=2)
		},
		{
			name:     "URL with too many query params",
			url:      "https://api.example.com/v1/users?a=1&b=2&c=3&d=4&e=5",
			expected: false, // query=5 (fails <=3)
		},
		{
			name:     "URL with wrong subdomain count",
			url:      "https://cdn.api.example.com/v1/users?id=1",
			expected: false, // subdomain=2 (fails ==1)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			result := validator.ValidateURL(parsedURL)
			if result != tt.expected {
				t.Errorf("For URL '%s', expected %t, got %t", tt.url, tt.expected, result)
			}
		})
	}
}

func TestDepthFilterValidatorORLogic(t *testing.T) {
	validator, err := NewDepthFilterValidator(
		[]string{">=4"},
		[]string{">=3"},
		[]string{">=2"},
		true, // OR logic
	)
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name     string
		url      string
		expected bool
	}{
		{
			name:     "URL matching path filter only",
			url:      "https://example.com/a/b/c/d/e", // path=5, query=0, subdomain=0
			expected: true,                           // passes path filter (>=4)
		},
		{
			name:     "URL matching query filter only",
			url:      "https://example.com/?a=1&b=2&c=3&d=4", // path=0, query=4, subdomain=0
			expected: true,                                    // passes query filter (>=3)
		},
		{
			name:     "URL matching subdomain filter only",
			url:      "https://a.b.example.com/", // path=0, query=0, subdomain=2
			expected: true,                       // passes subdomain filter (>=2)
		},
		{
			name:     "URL matching no filters",
			url:      "https://example.com/api", // path=1, query=0, subdomain=0
			expected: false,                     // fails all filters
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := url.Parse(tt.url)
			if err != nil {
				t.Fatalf("Failed to parse URL: %v", err)
			}

			result := validator.ValidateURL(parsedURL)
			if result != tt.expected {
				t.Errorf("For URL '%s', expected %t, got %t", tt.url, tt.expected, result)
			}
		})
	}
}