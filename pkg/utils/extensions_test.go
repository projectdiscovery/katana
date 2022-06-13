package utils

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestExtensionValidatorValidate(t *testing.T) {
	validator := NewExtensionValidator([]string{".go"}, nil, nil)
	require.True(t, validator.ValidatePath("main.go"), "could not validate correct data with extensions")
	require.False(t, validator.ValidatePath("main.php"), "could not validate correct data with wrong extension")

	// Check for wildcard
	validator = NewExtensionValidator([]string{"*"}, nil, nil)
	require.True(t, validator.ValidatePath("main.php"), "could not validate correct data with wildcard")

	validator = NewExtensionValidator(nil, nil, []string{".php"})
	require.False(t, validator.ValidatePath("main.php"), "could not validate correct data with deny list extension")
	require.True(t, validator.ValidatePath("main.go"), "could not validate correct data with no custom extensions")

	validator = NewExtensionValidator(nil, []string{".png"}, nil)
	require.True(t, validator.ValidatePath("main.png"), "could not validate correct data with default denylist bypass")
}
