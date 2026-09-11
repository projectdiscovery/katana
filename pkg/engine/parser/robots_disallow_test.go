package parser

import (
	"strings"
	"testing"
)

func TestRobotsTxtDisallowRuleParsing(t *testing.T) {
	robotsTxt := "User-agent: *
Disallow: /admin/
Disallow: /private/"
	lines := strings.Split(robotsTxt, "
")
	
	disallowCount := 0
	for _, l := range lines {
		if strings.HasPrefix(l, "Disallow:") {
			disallowCount++
		}
	}
	
	if disallowCount != 2 {
		t.Fatalf("expected 2 Disallow rules, found %d", disallowCount)
	}
}
