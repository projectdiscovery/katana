package extensions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatorValidate(t *testing.T) {
	validator := NewValidator([]string{".go"}, nil)
	require.True(t, validator.ValidatePath("main.go"), "could not validate correct data with extensions")
	require.False(t, validator.ValidatePath("main.php"), "could not validate correct data with wrong extension")

	validator = NewValidator(nil, []string{".php"})
	require.False(t, validator.ValidatePath("main.php"), "could not validate correct data with deny list extension")
	require.True(t, validator.ValidatePath("main.go"), "could not validate correct data with no custom extensions")

	validator = NewValidator([]string{"png"}, nil)
	require.True(t, validator.ValidatePath("main.png"), "could not validate correct data with default denylist bypass")

	// Test domain without extension
	validator = NewValidator([]string{".php"}, nil)
	require.True(t, validator.ValidatePath("https://browserstack.com"), "should allow root domain even with extension matching")

	// Test URL with query parameters
	require.False(t, validator.ValidatePath("https://browserstack.com/page.php?id=1"), "should not validate URL with non-matching extension")
	require.True(t, validator.ValidatePath("https://browserstack.com/page.php/?id=1"), "should allow directory paths even with extension")

	// Test URL with path but no extension
	validator = NewValidator([]string{".html"}, nil)
	require.False(t, validator.ValidatePath("https://browserstack.com/api/v1"), "should not match path without extension when extension list is specified")

	// Test root domain with extension matching
	validator = NewValidator([]string{".js"}, nil)
	require.True(t, validator.ValidatePath("https://example.com"), "should allow root domain even when extension matching is enabled")
	require.True(t, validator.ValidatePath("https://example.com/script.js"), "should allow matching extension")
	require.False(t, validator.ValidatePath("https://example.com/page.html"), "should not allow non-matching extension")

	// Test URLs with trailing slashes
	validator = NewValidator([]string{".js"}, nil)
	require.True(t, validator.ValidatePath("https://example.com/"), "should allow root domain with trailing slash when extension matching is enabled")
	require.True(t, validator.ValidatePath("https://example.com"), "should allow root domain without trailing slash when extension matching is enabled")
	require.True(t, validator.ValidatePath("https://example.com/js/"), "should allow path without extension when it's a directory")
	require.True(t, validator.ValidatePath("https://example.com/script.js/"), "should handle extension correctly even with trailing slash")
}

func TestValidatorExactMatch(t *testing.T) {
	// Test exact extension matching
	validator := NewValidator([]string{".js"}, nil)

	// URLs that should match
	require.True(t, validator.ExactMatch("https://example.com/script.js"), "should match .js file")
	require.True(t, validator.ExactMatch("https://example.com/js/jquery.js"), "should match .js file in subdirectory")
	require.True(t, validator.ExactMatch("main.js"), "should match local .js file")

	// URLs that should not match
	require.False(t, validator.ExactMatch("https://example.com"), "should not match root domain")
	require.False(t, validator.ExactMatch("https://example.com/"), "should not match root domain with slash")
	require.False(t, validator.ExactMatch("https://example.com/js/"), "should not match directory")
	require.False(t, validator.ExactMatch("https://example.com/page.html"), "should not match non-js file")
	// require.False(t, validator.ExactMatch("https://example.com/js/script.js/"), "should not match file with trailing slash")

	// Test with no extensions specified
	validator = NewValidator(nil, nil)
	require.True(t, validator.ExactMatch("https://example.com/any.js"), "should match any file when no extensions specified")
}
