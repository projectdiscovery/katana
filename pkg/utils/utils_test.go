package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseLinkTag(t *testing.T) {
	header := "<https://api.github.com/user/58276/repos?page=2>; rel=\"next\"," +
		"<https://api.github.com/user/58276/repos?page=10>; rel=\"last\""

	values := ParseLinkTag(header)
	require.ElementsMatch(t, []string{"https://api.github.com/user/58276/repos?page=2", "https://api.github.com/user/58276/repos?page=10"}, values, "could not parse correct links")
}

func TestParseRefreshTag(t *testing.T) {
	header := "999; url=/test/headers/refresh.found"

	values := ParseRefreshTag(header)
	require.Equal(t, "/test/headers/refresh.found", values, "could not parse correct links")

	tests := []struct {
		header   string
		expected string
	}{
		{header: "0; URL=/upper", expected: "/upper"},
		{header: "0;Url=/mixed", expected: "/mixed"},
		{header: "5; url = /spaced", expected: "/spaced"},
		{header: "0; url='/single-quoted'", expected: "/single-quoted"},
		{header: `0; url="/double-quoted"`, expected: "/double-quoted"},
		{header: "0; url=/login?url=/home", expected: "/login?url=/home"},
		{header: "0; url=", expected: ""},
		{header: "10", expected: ""},
	}
	for _, tt := range tests {
		require.Equal(t, tt.expected, ParseRefreshTag(tt.header), "could not parse refresh header %q", tt.header)
	}
}

func TestExtractParentPaths(t *testing.T) {
	urls := ExtractParentPaths("https://example.com/test/path/to/file.html")
	require.ElementsMatch(t, []string{"https://example.com/test/path/to", "https://example.com/test/path", "https://example.com/test"}, urls, "could not extract correct parent paths")
}
