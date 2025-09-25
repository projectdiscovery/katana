package normalizer

import (
	"strings"
	"testing"
)

func TestTextNormalizer_AllPatterns(t *testing.T) {
	normalizer, err := NewTextNormalizer()
	if err != nil {
		t.Fatalf("Failed to create normalizer: %v", err)
	}

	testText := `
		Contact us at test@example.com or admin@SITE.ORG for support.
		Server IP: 192.168.1.1 and public IP: 8.8.8.8
		Invalid IPs should not match: 999.999.999.999 or 300.400.500.600
		UUID: 550e8400-e29b-41d4-a716-446655440000
		Relative dates: 5 days ago, 2 weeks from now, 10 months ago
		Prices: $19.99, €50.00, £25.50, ¥1000
		Phone numbers: +1234567890, +447911123456, +33123456789
		SSN: 123-45-6789, 987-65-4321
		Timestamps: 2023-12-25 14:30:00, 12/25/2023 09:15:30
	`

	result := normalizer.Apply(testText)

	// Check that sensitive data was removed
	testCases := []struct {
		pattern         string
		shouldBeRemoved bool
		description     string
	}{
		{"test@example.com", true, "lowercase email"},
		{"admin@SITE.ORG", true, "uppercase email"},
		{"192.168.1.1", true, "private IP"},
		{"8.8.8.8", true, "public IP"},
		{"999.999.999.999", false, "invalid IP (too high octets)"},
		{"300.400.500.600", false, "invalid IP (too high octets)"},
		{"550e8400-e29b-41d4-a716-446655440000", true, "UUID"},
		{"5 days ago", true, "relative date - days ago"},
		{"2 weeks from now", true, "relative date - weeks from now"},
		{"10 months ago", true, "relative date - months ago"},
		{"$19.99", true, "USD price"},
		{"€50.00", true, "EUR price"},
		{"£25.50", true, "GBP price"},
		{"¥1000", true, "JPY price"},
		{"+1234567890", true, "international phone"},
		{"+447911123456", true, "UK phone"},
		{"+33123456789", true, "French phone"},
		{"123-45-6789", true, "SSN format 1"},
		{"987-65-4321", true, "SSN format 2"},
		{"2023-12-25 14:30:00", true, "ISO timestamp"},
		{"12/25/2023 09:15:30", true, "US timestamp"},
	}

	for _, tc := range testCases {
		if tc.shouldBeRemoved {
			if strings.Contains(result, tc.pattern) {
				t.Errorf("%s: Pattern '%s' should have been removed but was found in result", tc.description, tc.pattern)
			}
		} else {
			if !strings.Contains(result, tc.pattern) {
				t.Errorf("%s: Invalid pattern '%s' should not have been removed but was not found in result", tc.description, tc.pattern)
			}
		}
	}

	t.Logf("Original text length: %d", len(testText))
	t.Logf("Normalized text length: %d", len(result))
	t.Logf("Normalized result: %s", result)
}
