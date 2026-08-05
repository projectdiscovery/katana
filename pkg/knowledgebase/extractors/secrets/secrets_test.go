package secrets

import (
	"strings"
	"testing"
)

func TestExtract_DetectsKnownCredentials(t *testing.T) {
	e, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = e.Close() }()

	body := `<script>
		const k = "AKIAIOSFODNN7EXAMPLE";
		const s = "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY";
	</script>`

	out := e.Extract(body, nil, nil)
	if out == nil {
		t.Fatal("expected findings map, got nil")
	}
	findings, ok := out["findings"].([]map[string]any)
	if !ok || len(findings) == 0 {
		t.Fatalf("expected non-empty findings slice, got %#v", out)
	}

	var sawAWS bool
	for _, f := range findings {
		kind, _ := f["kind"].(string)
		if strings.Contains(strings.ToLower(kind), "aws") {
			sawAWS = true
			break
		}
	}
	if !sawAWS {
		t.Errorf("expected an AWS finding, got %#v", findings)
	}
}

func TestExtract_EmptyBody(t *testing.T) {
	e, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = e.Close() }()

	if got := e.Extract("", nil, nil); got != nil {
		t.Errorf("expected nil for empty body, got %#v", got)
	}
}

func TestExtract_NoSecrets(t *testing.T) {
	e, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = e.Close() }()

	if got := e.Extract("<html><body>hello world</body></html>", nil, nil); got != nil {
		t.Errorf("expected nil for body without secrets, got %#v", got)
	}
}

func TestName(t *testing.T) {
	e, err := New(Config{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = e.Close() }()

	if got := e.Name(); got != Name {
		t.Errorf("Name() = %q, want %q", got, Name)
	}
}
