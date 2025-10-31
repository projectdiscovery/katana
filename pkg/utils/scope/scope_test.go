package scope

import (
	"testing"

	urlutil "github.com/projectdiscovery/utils/url"
	"github.com/stretchr/testify/require"
)

func TestManagerValidate(t *testing.T) {
	t.Run("url", func(t *testing.T) {
		manager, err := NewManager([]string{`example`}, []string{`logout\.php`}, "dn", false)
		require.NoError(t, err, "could not create scope manager")

		parsed, _ := urlutil.Parse("https://test.com/index.php/example")
		validated, err := manager.Validate(parsed.URL, "test.com")
		require.NoError(t, err, "could not validate url")
		require.True(t, validated, "could not get correct in-scope validation")

		parsed, _ = urlutil.Parse("https://test.com/logout.php")
		validated, err = manager.Validate(parsed.URL, "another.com")
		require.NoError(t, err, "could not validate url")
		require.False(t, validated, "could not get correct out-scope validation")
	})
	t.Run("host", func(t *testing.T) {
		t.Run("dn", func(t *testing.T) {
			manager, err := NewManager(nil, nil, "dn", false)
			require.NoError(t, err, "could not create scope manager")

			parsed, _ := urlutil.Parse("https://testanother.com/index.php")
			validated, err := manager.Validate(parsed.URL, "test.com")
			require.NoError(t, err, "could not validate host")
			require.True(t, validated, "could not get correct in-scope validation")
		})
		t.Run("rdn", func(t *testing.T) {
			manager, err := NewManager(nil, nil, "rdn", false)
			require.NoError(t, err, "could not create scope manager")

			parsed, _ := urlutil.Parse("https://subdomain.example.com/logout.php")
			validated, err := manager.Validate(parsed.URL, "example.com")
			require.NoError(t, err, "could not validate host")
			require.True(t, validated, "could not get correct in-scope validation")
		})
		t.Run("localhost", func(t *testing.T) {
			manager, err := NewManager(nil, nil, "rdn", false)
			require.NoError(t, err, "could not create scope manager")

			parsed, _ := urlutil.Parse("http://localhost:8082/logout.php")
			validated, err := manager.Validate(parsed.URL, "localhost")
			require.NoError(t, err, "could not validate host")
			require.True(t, validated, "could not get correct in-scope validation")
		})
		t.Run("fqdn", func(t *testing.T) {
			manager, err := NewManager(nil, nil, "fqdn", false)
			require.NoError(t, err, "could not create scope manager")

			parsed, _ := urlutil.Parse("https://test.com/index.php")
			validated, err := manager.Validate(parsed.URL, "test.com")
			require.NoError(t, err, "could not validate host")
			require.True(t, validated, "could not get correct in-scope validation")

			parsed, _ = urlutil.Parse("https://subdomain.example.com/logout.php")
			validated, err = manager.Validate(parsed.URL, "example.com")
			require.NoError(t, err, "could not validate host")
			require.False(t, validated, "could not get correct out-scope validation")

			parsed, _ = urlutil.Parse("https://example.com/logout.php")
			validated, err = manager.Validate(parsed.URL, "another.com")
			require.NoError(t, err, "could not validate host")
			require.False(t, validated, "could not get correct out-scope validation")
		})
	})
}

func TestGetDomainRDNandDN(t *testing.T) {
	rdn, dn, err := getDomainRDNandRDN("test.projectdiscovery.io")
	require.Nil(t, err, "could not get domain rdn and dn")
	require.Equal(t, "projectdiscovery.io", rdn, "could not get correct rdn")
	require.Equal(t, "projectdiscovery", dn, "could not get correct dn")
}

func TestNoScopeWithOutOfScope(t *testing.T) {
	t.Run("noScope with outOfScope rules", func(t *testing.T) {
		// Create manager with noScope=true and outOfScope patterns
		outOfScopePatterns := []string{
			`logout\.php`,
			`/admin/`,
			`\.js$`,
			`^https?://[^/]+/\?lang=[a-z]{2}`,
		}
		manager, err := NewManager(nil, outOfScopePatterns, "rdn", true)
		require.NoError(t, err, "could not create scope manager with noScope and outOfScope")

		// Test 1: URL from different domain should be allowed (noScope ignores DNS)
		parsed, _ := urlutil.Parse("https://completely-different.com/index.php")
		validated, err := manager.Validate(parsed.URL, "original.com")
		require.NoError(t, err, "could not validate cross-domain URL with noScope")
		require.True(t, validated, "cross-domain URL should be allowed with noScope")

		// Test 2: URL matching outOfScope pattern should be rejected
		parsed, _ = urlutil.Parse("https://completely-different.com/logout.php")
		validated, err = manager.Validate(parsed.URL, "original.com")
		require.NoError(t, err, "could not validate outOfScope URL")
		require.False(t, validated, "outOfScope pattern should still be applied with noScope")

		// Test 3: Normal URLs should be allowed
		parsed, _ = urlutil.Parse("https://any-site.com/products/item123")
		validated, err = manager.Validate(parsed.URL, "original.com")
		require.NoError(t, err, "could not validate normal URL")
		require.True(t, validated, "normal URLs should be allowed")
	})

	t.Run("noScope with both inScope and outOfScope", func(t *testing.T) {
		// Test combining noScope with both inScope and outOfScope
		inScopePatterns := []string{`/api/`, `/products/`}
		outOfScopePatterns := []string{`/api/internal/`, `\.css$`}

		manager, err := NewManager(inScopePatterns, outOfScopePatterns, "fqdn", true)
		require.NoError(t, err, "could not create manager with both scope types")

		// Should be allowed: matches inScope, doesn't match outOfScope
		parsed, _ := urlutil.Parse("https://external.com/api/users")
		validated, err := manager.Validate(parsed.URL, "original.com")
		require.NoError(t, err, "could not validate API endpoint")
		require.True(t, validated, "API endpoint should be allowed")

		// Should be rejected: matches both inScope and outOfScope (outOfScope wins)
		parsed, _ = urlutil.Parse("https://external.com/api/internal/secrets")
		validated, err = manager.Validate(parsed.URL, "original.com")
		require.NoError(t, err, "could not validate internal API")
		require.False(t, validated, "internal API should be excluded by outOfScope")

		// Should be rejected: doesn't match inScope
		parsed, _ = urlutil.Parse("https://external.com/about/company")
		validated, err = manager.Validate(parsed.URL, "original.com")
		require.NoError(t, err, "could not validate about page")
		require.False(t, validated, "about page should be rejected (not in inScope)")

		// Should be rejected: matches outOfScope
		parsed, _ = urlutil.Parse("https://external.com/styles/main.css")
		validated, err = manager.Validate(parsed.URL, "original.com")
		require.NoError(t, err, "could not validate CSS file")
		require.False(t, validated, "CSS files should be excluded")
	})
}
