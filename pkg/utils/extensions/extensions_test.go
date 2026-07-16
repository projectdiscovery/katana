package extensions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatorValidate(t *testing.T) {
	validator := NewValidator([]string{".go"}, nil, false)
	require.True(t, validator.ValidatePath("main.go"), "could not validate correct data with extensions")
	require.False(t, validator.ValidatePath("main.php"), "could not validate correct data with wrong extension")

	validator = NewValidator(nil, []string{".php"}, false)
	require.False(t, validator.ValidatePath("main.php"), "could not validate correct data with deny list extension")
	require.True(t, validator.ValidatePath("main.go"), "could not validate correct data with no custom extensions")

	validator = NewValidator([]string{"png"}, nil, false)
	require.True(t, validator.ValidatePath("main.png"), "could not validate correct data with default denylist bypass")

	validator = NewValidator(nil, nil, true)
	require.True(t, validator.ValidatePath("main.png"), "could not validate correct data with no default extension filter")

	validator = NewValidator(nil, []string{"png"}, true)
	require.False(t, validator.ValidatePath("main.png"), "could not validate correct data with no default extension filter and custom filter")
}

func TestValidatorExtensionMatchNone(t *testing.T) {
	t.Run("em without none rejects extensionless URLs", func(t *testing.T) {
		validator := NewValidator([]string{"css", "js"}, nil, false)
		require.True(t, validator.ValidatePath("https://example.com/style.css"))
		require.True(t, validator.ValidatePath("https://example.com/app.js"))
		require.False(t, validator.ValidatePath("https://example.com/page"))
		require.False(t, validator.ValidatePath("https://example.com/"))
	})

	t.Run("em with none accepts extensionless URLs", func(t *testing.T) {
		validator := NewValidator([]string{"css", "js", "none"}, nil, false)
		require.True(t, validator.ValidatePath("https://example.com/style.css"))
		require.True(t, validator.ValidatePath("https://example.com/app.js"))
		require.True(t, validator.ValidatePath("https://example.com/page"))
		require.True(t, validator.ValidatePath("https://example.com/"))
		require.False(t, validator.ValidatePath("https://example.com/image.png"))
	})

	t.Run("em with only none accepts only extensionless URLs", func(t *testing.T) {
		validator := NewValidator([]string{"none"}, nil, false)
		require.True(t, validator.ValidatePath("https://example.com/page"))
		require.True(t, validator.ValidatePath("https://example.com/"))
		require.False(t, validator.ValidatePath("https://example.com/style.css"))
		require.False(t, validator.ValidatePath("https://example.com/app.js"))
	})

	t.Run("none is case insensitive", func(t *testing.T) {
		validator := NewValidator([]string{"None"}, nil, false)
		require.True(t, validator.ValidatePath("https://example.com/page"))

		validator = NewValidator([]string{"NONE"}, nil, false)
		require.True(t, validator.ValidatePath("https://example.com/page"))
	})
}
