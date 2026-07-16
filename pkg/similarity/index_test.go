package similarity

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func productHTML(id int, blurb string) []byte {
	return htmlPage(
		"Home Shop Cart Account Help",
		fmt.Sprintf("Product %d %s Exclusive cotton shirt available in multiple sizes with free shipping worldwide today only", id, blurb),
	)
}

func blogHTML(title, body string) []byte {
	return htmlPage(
		"Home Shop Cart Account Help",
		fmt.Sprintf("%s %s", title, body),
	)
}

func newTestIndex(t *testing.T, cfg Config) *Index {
	t.Helper()
	idx, err := New(cfg)
	require.NoError(t, err)
	return idx
}

func TestIndex_SimHash_ClusterBudget(t *testing.T) {
	idx := newTestIndex(t, Config{Mode: ModeSimHash, HammingDistance: 3, Budget: 1})

	p1 := productHTML(1, "red")
	p2 := productHTML(2, "red") // near-identical template
	p3 := blogHTML(
		"Deep Dive Into Distributed Consensus Algorithms",
		"Raft Paxos Viewstamped Replication quorum election log replication safety liveness detailed analysis for engineers building reliable systems",
	)

	require.True(t, idx.Accept(p1), "first product should be accepted")
	require.False(t, idx.Accept(p2), "near-duplicate product should be filtered with budget 1")
	require.True(t, idx.Accept(p3), "unrelated blog should be accepted")

	st := idx.Stats()
	require.Equal(t, int64(3), st.Processed)
	require.Equal(t, int64(2), st.Accepted)
	require.Equal(t, int64(1), st.Filtered)
}

func TestIndex_SimHash_BudgetTwo(t *testing.T) {
	idx := newTestIndex(t, Config{Mode: ModeSimHash, HammingDistance: 3, Budget: 2})

	require.True(t, idx.Accept(productHTML(1, "blue")))
	require.True(t, idx.Accept(productHTML(2, "blue")), "second near-dupe allowed with budget 2")
	require.False(t, idx.Accept(productHTML(3, "blue")), "third near-dupe filtered")
}

func TestIndex_TFIDF_NearCopyFiltered_UnrelatedAccepted(t *testing.T) {
	idx := newTestIndex(t, Config{Mode: ModeTFIDF, ScoreThreshold: 0.85, Budget: 1})

	a := blogHTML(
		"Introducing GraphQL Federation at Scale",
		"This article explains schema stitching gateway composition subgraph ownership and query planning challenges when operating federated graphs across many teams",
	)
	b := blogHTML(
		"Introducing GraphQL Federation at Scale",
		"This article explains schema stitching gateway composition subgraph ownership and query planning challenges when operating federated graphs across many teams with minor notes",
	)
	c := blogHTML(
		"Sourdough Baking Hydration Percentages",
		"Learn how flour water salt and starter ratios affect crumb structure oven spring and fermentation timing for artisan loaves baked on a stone",
	)

	require.True(t, idx.Accept(a))
	require.False(t, idx.Accept(b), "near-copy should exceed TF-IDF threshold")
	require.True(t, idx.Accept(c), "topically different page should pass")
}

func TestIndex_BM25_NearCopyFiltered_UnrelatedAccepted(t *testing.T) {
	idx := newTestIndex(t, Config{Mode: ModeBM25, ScoreThreshold: 0.85, Budget: 1})

	a := blogHTML(
		"Kubernetes Network Policies Deep Dive",
		"Calico Cilium eBPF egress ingress selectors namespaces and troubleshooting connection refused errors in multi-tenant clusters explained step by step",
	)
	b := blogHTML(
		"Kubernetes Network Policies Deep Dive",
		"Calico Cilium eBPF egress ingress selectors namespaces and troubleshooting connection refused errors in multi-tenant clusters explained step by step again",
	)
	c := blogHTML(
		"Classical Guitar Fingerstyle Patterns",
		"Travis picking arpeggios rest stroke free stroke practice routines metronome technique and repertoire suggestions for intermediate players",
	)

	require.True(t, idx.Accept(a))
	require.False(t, idx.Accept(b))
	require.True(t, idx.Accept(c))
}

func TestIndex_AllModes_LowSignalAlwaysAccepted(t *testing.T) {
	for _, mode := range []Mode{ModeSimHash, ModeTFIDF, ModeBM25} {
		t.Run(string(mode), func(t *testing.T) {
			idx := newTestIndex(t, Config{Mode: mode, Budget: 1})
			require.True(t, idx.Accept([]byte("ok")))
			require.True(t, idx.Accept([]byte("ok")))
		})
	}
}

func TestIndex_DisabledNilAccepts(t *testing.T) {
	var idx *Index
	require.True(t, idx.Accept([]byte("anything at all here for safety")))
}

func TestParseMode(t *testing.T) {
	m, err := ParseMode("TFIDF")
	require.NoError(t, err)
	require.Equal(t, ModeTFIDF, m)

	_, err = ParseMode("magic")
	require.Error(t, err)
}

func TestIndex_SharedChromeDoesNotCollapseDistinctMains(t *testing.T) {
	// Same nav/footer chrome; clearly different main content.
	for _, mode := range []Mode{ModeSimHash, ModeTFIDF, ModeBM25} {
		t.Run(string(mode), func(t *testing.T) {
			idx := newTestIndex(t, Config{
				Mode:            mode,
				HammingDistance: 3,
				ScoreThreshold:  0.85,
				Budget:          1,
			})
			a := htmlPage(
				"Home About Contact Shared Nav Items",
				"Astronomy telescope aperture focal length mount equatorial tracking deep sky objects nebulae galaxies observing checklist",
			)
			b := htmlPage(
				"Home About Contact Shared Nav Items",
				"Mediterranean cooking olive oil garlic lemon herbs grilled vegetables seafood pasta recipes weeknight dinners",
			)
			require.True(t, idx.Accept(a))
			require.True(t, idx.Accept(b), "distinct main content must not be collapsed by shared chrome")
		})
	}
}

func TestLexicalCorpus_EvictRepairsDocFreq(t *testing.T) {
	c := newLexicalCorpus()
	id0 := c.Add([]string{"alpha", "beta", "gamma", "delta", "epsilon"})
	id1 := c.Add([]string{"alpha", "zeta", "eta", "theta", "iota"})
	require.Equal(t, uint64(0), id0)
	require.Equal(t, uint64(1), id1)
	require.Equal(t, 2, c.docFreq["alpha"])
	evicted, ok := c.EvictOldest()
	require.True(t, ok)
	require.Equal(t, id0, evicted)
	require.Equal(t, 1, c.docFreq["alpha"])
	require.Equal(t, 0, c.docFreq["beta"])
	require.Equal(t, 1, c.Len())
	// Remaining doc keeps its stable ID.
	score, matchedID, ok := c.MaxCosine([]string{"alpha", "zeta", "eta", "theta", "iota"})
	require.True(t, ok)
	require.Equal(t, id1, matchedID)
	require.Greater(t, score, 0.9)
}

func TestIndex_StableClusterIDsSurviveEviction(t *testing.T) {
	for _, mode := range []Mode{ModeSimHash, ModeTFIDF, ModeBM25} {
		t.Run(string(mode), func(t *testing.T) {
			idx := newTestIndex(t, Config{
				Mode:            mode,
				HammingDistance: 3,
				ScoreThreshold:  0.85,
				Budget:          1,
				MaxDocuments:    2,
			})

			pages := [][]byte{
				blogHTML("Topic Alpha One", "alpha content about networking switches routers firewalls packet capture tooling"),
				blogHTML("Topic Beta Two", "beta content about photography lenses apertures shutters composition lighting"),
				blogHTML("Topic Gamma Three", "gamma content about baking sourdough hydration fermentation oven spring crumb"),
			}
			require.True(t, idx.Accept(pages[0]))
			require.True(t, idx.Accept(pages[1]))
			// Evicts the oldest representative (page 0).
			require.True(t, idx.Accept(pages[2]))

			// Near-copy of the evicted page must be treated as a new cluster again,
			// not reuse a shifted slice index / stale budget counter.
			nearEvicted := blogHTML("Topic Alpha One", "alpha content about networking switches routers firewalls packet capture tooling notes")
			require.True(t, idx.Accept(nearEvicted), "evicted cluster should not keep a stale budget")

			// Exact repeat of a still-resident representative must still be filtered.
			require.False(t, idx.Accept(pages[2]), "resident cluster budget must still apply")
		})
	}
}

func TestNormalize_MultiArticleWithoutMainUsesAllArticles(t *testing.T) {
	pageA := []byte(`<!doctype html><html><body>
<article>Shared lead story about space telescopes and deep sky surveys with nebulae catalogs</article>
<article>Unique page one second story about urban cycling cargo bikes commuting safety</article>
</body></html>`)
	pageB := []byte(`<!doctype html><html><body>
<article>Shared lead story about space telescopes and deep sky surveys with nebulae catalogs</article>
<article>Unique page two second story about sourdough baking hydration fermentation crumb</article>
</body></html>`)

	a, ok := Normalize(pageA)
	require.True(t, ok)
	b, ok := Normalize(pageB)
	require.True(t, ok)
	joinedA := strings.Join(a.Tokens, " ")
	joinedB := strings.Join(b.Tokens, " ")
	require.Contains(t, joinedA, "cycling")
	require.Contains(t, joinedB, "sourdough")
	require.NotEqual(t, joinedA, joinedB)
}
