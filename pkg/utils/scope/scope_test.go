package scope

import (
	"net/url"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestManagerValidate(t *testing.T) {
	t.Run("url", func(t *testing.T) {
		manager, err := NewManager([]string{`index\.php`}, []string{`logout\.php`}, true, false)
		require.NoError(t, err, "could not create scope manager")

		parsed, _ := url.Parse("https://test.com/index.php")
		validated, err := manager.Validate(parsed, "test.com")
		require.NoError(t, err, "could not validate url")
		require.True(t, validated, "could not get correct in-scope validation")

		parsed, _ = url.Parse("https://test.com/logout.php")
		validated, err = manager.Validate(parsed, "")
		require.NoError(t, err, "could not validate url")
		require.False(t, validated, "could not get correct out-scope validation")
	})
	t.Run("host", func(t *testing.T) {
		manager, err := NewManager(nil, nil, true, false)
		require.NoError(t, err, "could not create scope manager")

		parsed, _ := url.Parse("https://test.com/index.php")
		validated, err := manager.Validate(parsed, "test.com")
		require.NoError(t, err, "could not validate host")
		require.True(t, validated, "could not get correct in-scope validation")

		parsed, _ = url.Parse("https://example.test.com/index.php")
		validated, err = manager.Validate(parsed, "test.com")
		require.NoError(t, err, "could not validate host")
		require.True(t, validated, "could not get correct in-scope validation")

		parsed, _ = url.Parse("https://example.com/logout.php")
		validated, err = manager.Validate(parsed, "")
		require.NoError(t, err, "could not validate host")
		require.False(t, validated, "could not get correct out-scope validation")
	})
}
