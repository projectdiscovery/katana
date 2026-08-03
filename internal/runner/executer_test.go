package runner

import "testing"

func TestFormatCompletionStats(t *testing.T) {
	tests := []struct {
		name          string
		elapsed       string
		targetCount   int
		endpointCount int64
		expected      string
	}{
		{
			name:          "single target",
			elapsed:       "2s",
			targetCount:   1,
			endpointCount: 1,
			expected:      "Crawl completed in 2s for 1 target. 1 endpoint found.",
		},
		{
			name:          "multiple targets",
			elapsed:       "1m 5s",
			targetCount:   3,
			endpointCount: 128,
			expected:      "Crawl completed in 1m 5s for 3 targets. 128 endpoints found.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			actual := formatCompletionStats(test.elapsed, test.targetCount, test.endpointCount)
			if actual != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}
