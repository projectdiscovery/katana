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
}

func TestExtractParentPaths(t *testing.T) {
	urls := ExtractParentPaths("https://example.com/test/path/to/file.html")
	require.ElementsMatch(t, []string{"https://example.com/test/path/to", "https://example.com/test/path", "https://example.com/test"}, urls, "could not extract correct parent paths")
}
