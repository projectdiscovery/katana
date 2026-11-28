package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStatsLine(t *testing.T) {
	start := time.Date(2026, 11, 3, 13, 0, 0, 0, time.UTC)
	require.Equal(t, "[2026-11-03 14:01:05] [1:01:05] | Inputs: 1/3 (33%) | Endpoints: 120",
		statsLine(start.Add(time.Hour+65*time.Second), start, 1, 3, 120))
	require.Equal(t, "[2026-11-03 13:00:00] [0:00:00] | Inputs: 0/0 (0%) | Endpoints: 0",
		statsLine(start, start, 0, 0, 0))
}
