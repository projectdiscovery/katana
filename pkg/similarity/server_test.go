package similarity

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	similarProductCount = 300
	similarArticleCount = 150
	distinctPageCount   = 5

	// Shared product/article boilerplate must dominate the varying id token so
	// near-duplicate detection stays stable across hundreds of pages.
	productBoilerplate = `Exclusive cotton shirt available in multiple sizes with free shipping worldwide today only.
Soft fabric durable stitching classic fit warehouse clearance event premium materials carefully selected
for comfort durability and everyday wear across seasons. Customer reviews highlight consistent sizing
and colorfast dyes after repeated washing cycles. Pair with denim chinos or layered autumn jackets.
Care instructions recommend cold wash gentle cycle and line dry to preserve fabric integrity.
Available colors include navy charcoal olive sand and classic white with seasonal limited editions.`

	articleBoilerplate = `Deep analysis of distributed systems consensus protocols covering raft paxos viewstamped replication
quorum elections log replication safety and liveness guarantees for reliable platform engineers.
We examine leader election timeout tuning snapshotting membership changes and multi-region failover
strategies under partition scenarios. Operational playbooks include metrics dashboards alerting
thresholds and chaos experiments that validate recovery objectives. Readers should leave with a
practical checklist for production consensus deployments and capacity planning guidance.`
)

// similarPagesServer serves hundreds of template-similar product/article pages
// plus a handful of topically distinct pages.
func similarPagesServer(t *testing.T, products, articles, distinct int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		var b strings.Builder
		b.WriteString(`<!doctype html><html><body><nav>Home</nav><main><ul>`)
		for i := 1; i <= products; i++ {
			fmt.Fprintf(&b, `<li><a href="/product/%d">Product %d</a></li>`, i, i)
		}
		for i := 1; i <= articles; i++ {
			fmt.Fprintf(&b, `<li><a href="/article/%d">Article %d</a></li>`, i, i)
		}
		for i := 1; i <= distinct; i++ {
			fmt.Fprintf(&b, `<li><a href="/distinct/%d">Distinct %d</a></li>`, i, i)
		}
		b.WriteString(`</ul></main></body></html>`)
		_, _ = io.WriteString(w, b.String())
	})

	mux.HandleFunc("/product/", func(w http.ResponseWriter, r *http.Request) {
		_ = strings.TrimPrefix(r.URL.Path, "/product/")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// Body is intentionally identical across product IDs (id lives in the URL
		// only) so hundreds of template pages form one near-duplicate cluster.
		_, _ = fmt.Fprintf(w, `<!doctype html><html><body>
<nav>Home Products Cart Help Account</nav>
<main>Product detail. %s</main>
<footer>copyright 2026 example corp all rights reserved</footer>
</body></html>`, productBoilerplate)
	})

	mux.HandleFunc("/article/", func(w http.ResponseWriter, r *http.Request) {
		_ = strings.TrimPrefix(r.URL.Path, "/article/")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!doctype html><html><body>
<nav>Home Blog Topics Authors Subscribe</nav>
<main>Article detail. %s</main>
<footer>copyright 2026 example corp all rights reserved</footer>
</body></html>`, articleBoilerplate)
	})

	mux.HandleFunc("/distinct/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/distinct/")
		n, _ := strconv.Atoi(id)
		pages := []string{
			`Mediterranean cooking workshop focusing on olive oil tasting garlic confit lemon zest herb blends grilled vegetables seafood stews handmade pasta dough and weeknight dinner planning for families who love bright coastal flavors.`,
			`Amateur astronomy field guide covering telescope aperture choices focal length ratios equatorial mounts nebulae catalogs galaxy hunting polar alignment and cold-weather observing checklists for dark-sky weekend trips.`,
			`Classical guitar practice curriculum with travis picking patterns rest-stroke free-stroke drills metronome ladders repertoire goals for bach loure studies and performance anxiety routines for intermediate players.`,
			`Sourdough bakery notebook documenting hydration percentages flour blends water temperature salt ratios starter feeding schedules crumb structure scoring patterns oven spring targets and fermentation timelines for artisan loaves.`,
			`Urban cycling advocacy handbook about cargo bike geometry lighting standards U-lock technique commuting rain gear protected intersections and city infrastructure campaigns that improve everyday rider safety.`,
		}
		body := pages[(n-1)%len(pages)]
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprintf(w, `<!doctype html><html><body>
<nav>Home Products Cart Help Account</nav>
<main>%s</main>
<footer>copyright 2026 example corp all rights reserved</footer>
</body></html>`, body)
	})

	return httptest.NewServer(mux)
}

func fetchBody(t *testing.T, client *http.Client, url string) []byte {
	t.Helper()
	resp, err := client.Get(url)
	require.NoError(t, err)
	defer resp.Body.Close() //nolint:errcheck
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	return body
}

type crawlStats struct {
	productAccepted  int
	productFiltered  int
	articleAccepted  int
	articleFiltered  int
	distinctAccepted int
	distinctFiltered int
	elapsed          time.Duration
}

func (s crawlStats) totalAccepted() int {
	return s.productAccepted + s.articleAccepted + s.distinctAccepted
}

func (s crawlStats) totalFiltered() int {
	return s.productFiltered + s.articleFiltered + s.distinctFiltered
}

func (s crawlStats) totalProcessed() int {
	return s.totalAccepted() + s.totalFiltered()
}

func runSimilarSiteCrawl(t *testing.T, idx *Index, server *httptest.Server, products, articles, distinct int) crawlStats {
	t.Helper()
	client := server.Client()
	start := time.Now()
	var st crawlStats

	for i := 1; i <= products; i++ {
		body := fetchBody(t, client, fmt.Sprintf("%s/product/%d", server.URL, i))
		if idx.Accept(body) {
			st.productAccepted++
		} else {
			st.productFiltered++
		}
	}
	for i := 1; i <= articles; i++ {
		body := fetchBody(t, client, fmt.Sprintf("%s/article/%d", server.URL, i))
		if idx.Accept(body) {
			st.articleAccepted++
		} else {
			st.articleFiltered++
		}
	}
	for i := 1; i <= distinct; i++ {
		body := fetchBody(t, client, fmt.Sprintf("%s/distinct/%d", server.URL, i))
		if idx.Accept(body) {
			st.distinctAccepted++
		} else {
			st.distinctFiltered++
		}
	}

	st.elapsed = time.Since(start)
	return st
}

func assertIndexStatsMatch(t *testing.T, idx *Index, got crawlStats) {
	t.Helper()
	st := idx.Stats()
	require.Equal(t, int64(got.totalProcessed()), st.Processed, "index Processed")
	require.Equal(t, int64(got.totalAccepted()), st.Accepted, "index Accepted")
	require.Equal(t, int64(got.totalFiltered()), st.Filtered, "index Filtered")
	require.Equal(t, st.Processed, st.Accepted+st.Filtered, "Accepted+Filtered must equal Processed")
}

func logCrawlStats(t *testing.T, mode Mode, budget int, got crawlStats) {
	t.Helper()
	filterRate := 100 * float64(got.totalFiltered()) / float64(got.totalProcessed())
	t.Logf("[%s budget=%d] processed=%d accepted=%d filtered=%d (%.1f%% filtered) in %s",
		mode, budget, got.totalProcessed(), got.totalAccepted(), got.totalFiltered(), filterRate, got.elapsed.Round(time.Millisecond))
	t.Logf("  products: accepted=%d filtered=%d | articles: accepted=%d filtered=%d | distinct: accepted=%d filtered=%d",
		got.productAccepted, got.productFiltered, got.articleAccepted, got.articleFiltered, got.distinctAccepted, got.distinctFiltered)
}

func TestSimilarPagesServer_HundredsFilteredPerMode(t *testing.T) {
	products, articles, distinct := similarProductCount, similarArticleCount, distinctPageCount
	server := similarPagesServer(t, products, articles, distinct)
	defer server.Close()

	totalPages := products + articles + distinct
	require.GreaterOrEqual(t, totalPages, 400, "fixture must exercise hundreds of pages")

	cases := []struct {
		name   string
		cfg    Config
		budget int
	}{
		{"simhash", Config{Mode: ModeSimHash, HammingDistance: 3, Budget: 1}, 1},
		{"tfidf", Config{Mode: ModeTFIDF, ScoreThreshold: 0.85, Budget: 1}, 1},
		{"bm25", Config{Mode: ModeBM25, ScoreThreshold: 0.85, Budget: 1}, 1},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			idx := newTestIndex(t, tc.cfg)
			got := runSimilarSiteCrawl(t, idx, server, products, articles, distinct)
			logCrawlStats(t, tc.cfg.Mode, tc.budget, got)

			// Two template clusters (products, articles), each keeps budget representatives.
			require.Equal(t, tc.budget, got.productAccepted, "product cluster representatives")
			require.Equal(t, products-tc.budget, got.productFiltered, "product near-dupes skipped")
			require.Equal(t, tc.budget, got.articleAccepted, "article cluster representatives")
			require.Equal(t, articles-tc.budget, got.articleFiltered, "article near-dupes skipped")

			// Distinct topical pages must survive.
			require.Equal(t, distinct, got.distinctAccepted, "all distinct pages accepted")
			require.Zero(t, got.distinctFiltered)

			expectedAccepted := 2*tc.budget + distinct
			expectedFiltered := (products - tc.budget) + (articles - tc.budget)
			require.Equal(t, expectedAccepted, got.totalAccepted())
			require.Equal(t, expectedFiltered, got.totalFiltered())
			require.Equal(t, totalPages, got.totalProcessed())

			// Vast majority of the site should be filtered as similar.
			require.Greater(t, got.totalFiltered(), got.totalAccepted()*10,
				"filtered volume should dwarf accepted representatives on a template-heavy site")
			assertIndexStatsMatch(t, idx, got)
		})
	}
}

func TestSimilarPagesServer_BudgetTwoScales(t *testing.T) {
	products, articles, distinct := similarProductCount, similarArticleCount, distinctPageCount
	server := similarPagesServer(t, products, articles, distinct)
	defer server.Close()

	const budget = 2
	for _, mode := range []Mode{ModeSimHash, ModeTFIDF, ModeBM25} {
		t.Run(string(mode), func(t *testing.T) {
			idx := newTestIndex(t, Config{
				Mode:            mode,
				HammingDistance: 3,
				ScoreThreshold:  0.85,
				Budget:          budget,
			})
			got := runSimilarSiteCrawl(t, idx, server, products, articles, distinct)
			logCrawlStats(t, mode, budget, got)

			require.Equal(t, budget, got.productAccepted)
			require.Equal(t, products-budget, got.productFiltered)
			require.Equal(t, budget, got.articleAccepted)
			require.Equal(t, articles-budget, got.articleFiltered)
			require.Equal(t, distinct, got.distinctAccepted)
			require.Zero(t, got.distinctFiltered)

			assertIndexStatsMatch(t, idx, got)
			require.Equal(t, int64(2*budget+distinct), idx.Stats().Accepted)
			require.Equal(t, int64(products+articles-2*budget), idx.Stats().Filtered)
		})
	}
}

func TestSimilarPagesServer_DisabledKeepsAllHundreds(t *testing.T) {
	products := similarProductCount
	server := similarPagesServer(t, products, 0, 0)
	defer server.Close()
	client := server.Client()

	var idx *Index
	accepted := 0
	for i := 1; i <= products; i++ {
		body := fetchBody(t, client, fmt.Sprintf("%s/product/%d", server.URL, i))
		if idx.Accept(body) {
			accepted++
		}
	}
	require.Equal(t, products, accepted, "nil index must not skip any of the %d similar pages", products)
}
