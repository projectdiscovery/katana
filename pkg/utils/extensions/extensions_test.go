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
