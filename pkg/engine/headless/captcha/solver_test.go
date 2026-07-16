package captcha

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewSolver_UnsupportedProvider(t *testing.T) {
	_, err := NewSolver("unknown-provider", "key")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsupported")
}

func TestRegisterSolver(t *testing.T) {
	RegisterSolver("test-provider", func(apiKey string) (Solver, error) {
		return nil, nil
	})
	defer delete(solverRegistry, "test-provider")

	_, ok := solverRegistry["test-provider"]
	assert.True(t, ok)
}
