package jsparser

import "testing"

func containsEndpoint(endpoints []Endpoint, target string) bool {
	for _, endpoint := range endpoints {
		if endpoint.Endpoint == target {
			return true
		}
	}
	return false
}

func TestExtractBasicPatterns(t *testing.T) {
	source := `
		const a = "/api/users";
		const b = "https://example.com/v1/login";
		const c = "wss://example.com/socket";
	`

	got := Extract(source)

	for _, expected := range []string{
		"/api/users",
		"https://example.com/v1/login",
	} {
		if !containsEndpoint(got, expected) {
			t.Fatalf("expected endpoint %q in %#v", expected, got)
		}
	}

	if Backend() == "goja" && !containsEndpoint(got, "wss://example.com/socket") {
		t.Fatalf("expected endpoint %q in %#v", "wss://example.com/socket", got)
	}
}

func TestExtractCallPatterns(t *testing.T) {
	source := `
		fetch("/api/users");
		axios.get("/v1/login");
		$.ajax("/graphql");
		jQuery.post("/submit");
	`

	got := Extract(source)

	for _, expected := range []string{
		"/api/users",
		"/v1/login",
		"/graphql",
		"/submit",
	} {
		if !containsEndpoint(got, expected) {
			t.Fatalf("expected endpoint %q in %#v", expected, got)
		}
	}
}

func TestExtractConcatenationAndIdentifiers(t *testing.T) {
	source := `
		const base = "/api";
		const users = base + "/users";
		fetch(users);
	`

	got := Extract(source)

	if Backend() == "goja" {
		for _, expected := range []string{"/api", "/api/users"} {
			if !containsEndpoint(got, expected) {
				t.Fatalf("expected endpoint %q in %#v", expected, got)
			}
		}
		return
	}

	for _, expected := range []string{"/api", "/users"} {
		if !containsEndpoint(got, expected) {
			t.Fatalf("expected endpoint %q in %#v", expected, got)
		}
	}
}

func TestExtractTemplateLiteral(t *testing.T) {
	source := "const base = \"/api\";\n" +
		"const version = \"/v1\";\n" +
		"const url = `${base}${version}/users`;\n" +
		"fetch(url);\n"

	got := Extract(source)

	if Backend() == "goja" {
		if !containsEndpoint(got, "/api/v1/users") {
			t.Fatalf("expected template literal endpoint in %#v", got)
		}
		return
	}

	for _, expected := range []string{"/api", "/v1"} {
		if !containsEndpoint(got, expected) {
			t.Fatalf("expected endpoint %q in %#v", expected, got)
		}
	}
}

