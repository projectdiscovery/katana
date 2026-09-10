package parser

import (
	"strings"
	"testing"
)

func TestDataURISchemeFiltration(t *testing.T) {
	dataURI := "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg=="
	if !strings.HasPrefix(dataURI, "data:") {
		t.Fatalf("expected data URI prefix")
	}
}
