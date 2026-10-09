package crawler

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
	"github.com/stretchr/testify/require"
)

func TestSamplePaths(t *testing.T) {
	t.Parallel()
	paths := []*types.Path{{TargetID: "a"}, {TargetID: "b"}, {TargetID: "c"}}
	require.Nil(t, samplePaths(paths, 0))
	require.Len(t, samplePaths(paths, 2), 2)
	require.Len(t, samplePaths(paths, 10), 3)
}
