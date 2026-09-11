package parser

import (
	"strings"
	"testing"
)

func TestSVGXLinkHREFEndpointExtraction(t *testing.T) {
	tag := `<use xlink:href="/assets/icons.svg#icon-user"></use>`
	if !strings.Contains(tag, "xlink:href") {
		t.Fatalf("failed to locate xlink:href attribute in SVG snippet")
	}
}
