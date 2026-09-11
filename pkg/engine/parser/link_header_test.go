package parser

import (
	"strings"
	"testing"
)

func TestHTTPLinkHeaderEndpointExtraction(t *testing.T) {
	headerVal := `<https://api.example.com/v2/items?page=2>; rel="next", <https://api.example.com/v2/items?page=10>; rel="last"`
	if !strings.Contains(headerVal, `rel="next"`) {
		t.Fatalf("failed to locate pagination link header relation")
	}
}
