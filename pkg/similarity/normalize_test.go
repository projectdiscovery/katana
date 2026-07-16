package similarity

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func htmlPage(nav, main string) []byte {
	return []byte(fmt.Sprintf(`<!doctype html><html><body>
<nav>%s</nav>
<main>%s</main>
<footer>copyright 2026 example corp</footer>
<script>var x = 1;</script>
</body></html>`, nav, main))
}

func TestNormalize_StripsChromeAndScripts(t *testing.T) {
	body := htmlPage(
		"Home About Contact Login",
		"Unique Product Alpha with rare widget terminology and packing details",
	)
	doc, ok := Normalize(body)
	require.True(t, ok)
	joined := strings.Join(doc.Tokens, " ")
	require.Contains(t, joined, "unique")
	require.Contains(t, joined, "product")
	require.NotContains(t, joined, "login")
	require.NotContains(t, joined, "copyright")
	require.NotContains(t, joined, "var")
}

func TestNormalize_PlainText(t *testing.T) {
	doc, ok := Normalize([]byte("Alpha beta gamma delta epsilon zeta eta"))
	require.True(t, ok)
	require.GreaterOrEqual(t, len(doc.Tokens), minTokensForSimilarity)
}

func TestNormalize_TooShort(t *testing.T) {
	_, ok := Normalize([]byte("hi"))
	require.False(t, ok)
}

func TestSimHash_SimilarTextClose_UnrelatedFar(t *testing.T) {
	base := tokenize("the quick brown fox jumps over the lazy dog again today while another fox waits beside the river bank under morning sunlight near the wooden bridge")
	edited := tokenize("the quick black fox jumps over the lazy dog again today while another fox waits beside the river bank under morning sunlight near the wooden bridge")
	unrelated := tokenize("kubernetes cluster orchestration scheduler pod node affinity ingress service mesh control plane etcd api server controller manager kubelet runtime")

	a := SimHash64(shingles(base, 3))
	b := SimHash64(shingles(edited, 3))
	c := SimHash64(shingles(unrelated, 3))

	require.Less(t, HammingDistance(a, b), HammingDistance(a, c), "small edit should be closer than unrelated text")
	require.Greater(t, HammingDistance(a, c), 10, "unrelated text should be far apart")
}

func TestSimHash_NearIdenticalDocumentsWithinDefaultBand(t *testing.T) {
	a, ok := Normalize(productHTML(1, "crimson"))
	require.True(t, ok)
	b, ok := Normalize(productHTML(2, "crimson"))
	require.True(t, ok)
	require.LessOrEqual(t, HammingDistance(SimHash64(a.Shingles), SimHash64(b.Shingles)), DefaultHammingDistance)
}

func TestSimHash_IdenticalZeroDistance(t *testing.T) {
	features := shingles(tokenize("alpha beta gamma delta epsilon zeta eta theta"), 3)
	h1 := SimHash64(features)
	h2 := SimHash64(features)
	require.Equal(t, 0, HammingDistance(h1, h2))
}
