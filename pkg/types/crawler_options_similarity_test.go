package types

import (
	"testing"

	"github.com/projectdiscovery/katana/pkg/similarity"
	"github.com/stretchr/testify/require"
)

func TestNewCrawlerOptions_ContentSimilaritySidecar(t *testing.T) {
	opts := &Options{
		MaxDepth:                       1,
		BodyReadSize:                   1024,
		Strategy:                       "depth-first",
		PageContentSimilar:             true,
		PageContentSimilarMode:         "bm25",
		PageContentSimilarDistance:     3,
		PageContentSimilarThresholdStr: "0.9",
		PageContentSimilarBudget:       2,
		FieldScope:                     "rdn",
	}
	co, err := NewCrawlerOptions(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = co.Close() })

	require.NotNil(t, co.UniqueFilter, "Simple filter must remain")
	require.NotNil(t, co.ContentSimilarity)
	require.Equal(t, similarity.ModeBM25, co.ContentSimilarity.Mode())
}

func TestNewCrawlerOptions_ContentSimilarityOffByDefault(t *testing.T) {
	opts := &Options{
		MaxDepth:     1,
		BodyReadSize: 1024,
		Strategy:     "depth-first",
		FieldScope:   "rdn",
	}
	co, err := NewCrawlerOptions(opts)
	require.NoError(t, err)
	t.Cleanup(func() { _ = co.Close() })
	require.Nil(t, co.ContentSimilarity)
}

func TestContentSimilarityEnabledAlias(t *testing.T) {
	require.True(t, (&Options{SimilarityDeduplication: true}).ContentSimilarityEnabled())
	require.True(t, (&Options{PageContentSimilar: true}).ContentSimilarityEnabled())
	require.False(t, (&Options{}).ContentSimilarityEnabled())
}
