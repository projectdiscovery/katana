package parser

import (
	"strings"
	"testing"
)

func TestMetaRefreshURLParsing(t *testing.T) {
	tag := `<meta http-equiv="refresh" content="5; url=https://example.com/login">`
	if !strings.Contains(tag, "url=") {
		t.Fatalf("failed to locate url parameter in meta refresh tag")
	}
}
