package extensions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatorValidate(t *testing.T) {
	// Test extension matching for crawling
	validator := NewValidator([]string{".go"}, nil)
	require.True(t, validator.ValidatePath("main.go"), "should allow files with matching extension")
	require.True(t, validator.ValidatePath("main.php"), "should allow files with non-matching extension for crawling")

	// Test deny list
	validator = NewValidator(nil, []string{".php"})
	// require.False(t, validator.ValidatePath("main.php"), "should not allow denied extensions")
	require.True(t, validator.ValidatePath("main.go"), "should allow non-denied extensions")

	// Test default denylist bypass with extension matching
	validator = NewValidator([]string{"png"}, nil)
	require.True(t, validator.ValidatePath("main.png"), "should allow specified extension even if in default denylist")

	// Test URLs with extensions
	validator = NewValidator([]string{".php"}, nil)
	require.True(t, validator.ValidatePath("https://example.com"), "should allow root domain for crawling")
	require.True(t, validator.ValidatePath("https://example.com/page.php?id=1"), "should allow matching extension with query params")

	// Test paths without extensions
	validator = NewValidator([]string{".html"}, nil)
	require.True(t, validator.ValidatePath("https://example.com/api/v1"), "should allow paths without extensions for crawling")

	// Test extension matching with different file types
	validator = NewValidator([]string{".js"}, nil)
	require.True(t, validator.ValidatePath("https://example.com"), "should allow root domain for crawling")
	require.True(t, validator.ValidatePath("https://example.com/script.js"), "should allow matching extension")

	// Test URLs with trailing slashes
	validator = NewValidator([]string{".js"}, nil)
	require.True(t, validator.ValidatePath("https://example.com/"), "should allow root domain with trailing slash when extension matching is enabled")
	require.True(t, validator.ValidatePath("https://example.com"), "should allow root domain without trailing slash when extension matching is enabled")
	require.True(t, validator.ValidatePath("https://example.com/js/"), "should allow path without extension when it's a directory")
	require.True(t, validator.ValidatePath("https://example.com/script.js/"), "should handle extension correctly even with trailing slash")
}

func TestValidatorExactMatch(t *testing.T) {
	// Test exact extension matching for output filtering
	validator := NewValidator([]string{".js"}, nil)

	// Files that should match (both URLs and local files)
	require.True(t, validator.ExactMatch("https://example.com/script.js"), "should match .js file in URL")
	require.True(t, validator.ExactMatch("https://example.com/js/jquery.js"), "should match .js file in subdirectory")
	require.True(t, validator.ExactMatch("main.js"), "should match local .js file")
	require.True(t, validator.ExactMatch("https://example.com/file.js?param=value"), "should match .js file with query parameters")

	// URLs that should not match
	require.False(t, validator.ExactMatch("https://example.com"), "should not match root domain")
	require.False(t, validator.ExactMatch("https://example.com/"), "should not match root domain with slash")
	require.False(t, validator.ExactMatch("https://example.com/js/"), "should not match directory")
	require.False(t, validator.ExactMatch("https://example.com/page.html"), "should not match non-js file")

	// Test with no extensions specified
	validator = NewValidator(nil, nil)
	require.True(t, validator.ExactMatch("https://example.com/any.js"), "should match any file when no extensions specified")
}
