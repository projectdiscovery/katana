package similarity

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCosine_IdenticalHigh_UnrelatedLow(t *testing.T) {
	c := newLexicalCorpus()
	c.Add(tokenize("graphql federation schema stitching gateway subgraph query planning ownership"))
	high, _, ok := c.MaxCosine(tokenize("graphql federation schema stitching gateway subgraph query planning ownership notes"))
	require.True(t, ok)
	low, _, ok := c.MaxCosine(tokenize("sourdough baking hydration flour water salt starter crumb oven spring"))
	require.True(t, ok)
	require.GreaterOrEqual(t, high, 0.85)
	require.Less(t, low, 0.5)
}

func TestBM25_IdenticalHigh_UnrelatedLow(t *testing.T) {
	c := newLexicalCorpus()
	c.Add(tokenize("kubernetes network policies calico cilium ebpf egress ingress selectors namespaces"))
	high, _, ok := c.MaxBM25(tokenize("kubernetes network policies calico cilium ebpf egress ingress selectors namespaces again"))
	require.True(t, ok)
	low, _, ok := c.MaxBM25(tokenize("classical guitar fingerstyle travis picking arpeggios metronome repertoire"))
	require.True(t, ok)
	require.GreaterOrEqual(t, high, 0.85)
	require.Less(t, low, 0.5)
}
