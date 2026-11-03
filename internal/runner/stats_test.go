package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStatsLine(t *testing.T) {
	require.Equal(t, "[1m 5s] Inputs: 1/3 (33%) | Endpoints: 120", statsLine(65*time.Second, 1, 3, 120))
	require.Equal(t, "[0ms] Inputs: 0/0 (0%) | Endpoints: 0", statsLine(0, 0, 0, 0))
}
