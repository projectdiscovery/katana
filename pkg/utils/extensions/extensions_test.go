package extensions

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidatorValidate(t *testing.T) {
	validator := NewValidator([]string{".go"}, nil, nil)
	require.True(t, validator.ValidatePath("main.go"), "could not validate correct data with extensions")
	require.False(t, validator.ValidatePath("main.php"), "could not validate correct data with wrong extension")

	// Check for wildcard
	validator = NewValidator([]string{"*"}, nil, nil)
	require.True(t, validator.ValidatePath("main.php"), "could not validate correct data with wildcard")

	validator = NewValidator(nil, nil, []string{".php"})
	require.False(t, validator.ValidatePath("main.php"), "could not validate correct data with deny list extension")
	require.True(t, validator.ValidatePath("main.go"), "could not validate correct data with no custom extensions")

	validator = NewValidator(nil, []string{".png"}, nil)
	require.True(t, validator.ValidatePath("main.png"), "could not validate correct data with default denylist bypass")
}
