// Package graph implements a Directed Graph for storing
// state information during crawling of a Web Application.
package graph

import (
	"os"
	"sync"

	"github.com/dominikbraun/graph"
	"github.com/dominikbraun/graph/draw"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// CrawlGraph is a graph for storing state information during crawling.
// All methods are safe for concurrent use.
type CrawlGraph struct {
	mu    sync.RWMutex
	graph graph.Graph[string, types.PageState]
}

func navigationHasherFunc(n types.PageState) string {
	return n.UniqueID
}

// NewCrawlGraph creates a new CrawlGraph instance
func NewCrawlGraph() *CrawlGraph {
	return &CrawlGraph{
		graph: graph.New(navigationHasherFunc, func(t *graph.Traits) {
			t.IsDirected = true
			t.IsRooted = true
			t.IsWeighted = true
		}),
	}
}

func (g *CrawlGraph) GetVertices() []string {
	g.mu.RLock()
	defer g.mu.RUnlock()

	vertices := []string{}
	adjacencyMap, err := g.graph.AdjacencyMap()
	if err != nil {
		return nil
	}
	for vertex := range adjacencyMap {
		vertices = append(vertices, vertex)
	}
	return vertices
}

// AddPageState adds a navigation to the graph.
func (g *CrawlGraph) AddPageState(n types.PageState) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	vertexAttrs := map[string]string{
		"label": n.URL,
	}
	if n.IsRoot {
		vertexAttrs["is_root"] = "true"
	}

	err := g.graph.AddVertex(n, func(vp *graph.VertexProperties) {
		vp.Weight = n.Depth
		vp.Attributes = vertexAttrs
	})
	if err != nil {
		if errors.Is(err, graph.ErrVertexAlreadyExists) {
			return nil
		}
		return errors.Wrap(err, "could not add vertex to graph")
	}

	if n.NavigationAction != nil {
		edgeAttrs := map[string]string{
			"label": n.NavigationAction.String(),
		}

		err = g.graph.AddEdge(n.OriginID, n.UniqueID, func(ep *graph.EdgeProperties) {
			ep.Weight = n.Depth
			ep.Attributes = edgeAttrs
		})
		if err != nil {
			if errors.Is(err, graph.ErrEdgeAlreadyExists) {
				return nil
			}
			return errors.Wrapf(err, "could not add edge to graph: source vertex %s", n.OriginID)
		}
	}
	return nil
}

func (g *CrawlGraph) AddEdge(sourceState, targetState string, action *types.Action) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if action == nil {
		return errors.New("add edge: action cannot be nil")
	}
	edgeAttrs := map[string]string{
		"label": action.String(),
	}
	err := g.graph.AddEdge(sourceState, targetState, func(ep *graph.EdgeProperties) {
		ep.Weight = action.Depth
		ep.Attributes = edgeAttrs
	})
	if err != nil {
		if errors.Is(err, graph.ErrEdgeAlreadyExists) {
			return nil
		}
		return errors.Wrap(err, "could not add edge to graph")
	}
	return nil
}

func (g *CrawlGraph) GetPageState(id string) (*types.PageState, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	pageVertex, err := g.graph.Vertex(id)
	if err != nil {
		return nil, errors.Wrap(err, "could not get vertex")
	}
	return &pageVertex, nil
}

func (g *CrawlGraph) ShortestPath(sourceState, targetState string) ([]*types.Action, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	shortestPath, err := graph.ShortestPath(g.graph, sourceState, targetState)
	if err != nil {
		return nil, errors.Wrap(err, "could not find shortest path")
	}
	actionsSlice := make([]*types.Action, 0, len(shortestPath))
	for _, path := range shortestPath {
		pageVertex, err := g.graph.Vertex(path)
		if err != nil {
			return nil, errors.Wrap(err, "could not get vertex")
		}

		if pageVertex.URL == "about:blank" || pageVertex.NavigationAction == nil {
			continue
		}
		actionsSlice = append(actionsSlice, pageVertex.NavigationAction)
	}
	return actionsSlice, nil
}

// GraphNode represents a page in the crawl graph for report export.
type GraphNode struct {
	ID    string `json:"id"`
	URL   string `json:"url"`
	Title string `json:"title,omitempty"`
	Depth int    `json:"depth"`
}

// GraphEdge represents a navigation between two pages.
type GraphEdge struct {
	From      string `json:"from"`       // source URL
	To        string `json:"to"`         // target URL
	Action    string `json:"action"`     // how we navigated (click, form submit, etc.)
}

// ExportGraph returns all nodes and edges with human-readable URLs (not hashes).
func (g *CrawlGraph) ExportGraph() (nodes []GraphNode, edges []GraphEdge) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	adjacencyMap, err := g.graph.AdjacencyMap()
	if err != nil {
		return nil, nil
	}

	// Collect all nodes, deduplicating by URL (multiple graph vertices
	// can point to the same URL when a page is reached from different sources).
	seenURL := make(map[string]bool)
	for vertexID := range adjacencyMap {
		vertex, err := g.graph.Vertex(vertexID)
		if err != nil {
			continue
		}
		if vertex.URL == "" || vertex.URL == "about:blank" {
			continue
		}
		if seenURL[vertex.URL] {
			continue
		}
		seenURL[vertex.URL] = true
		nodes = append(nodes, GraphNode{
			ID:    vertexID[:8], // short hash for display
			URL:   vertex.URL,
			Title: vertex.Title,
			Depth: vertex.Depth,
		})
	}

	// Collect all edges with URL labels
	for sourceID, targetMap := range adjacencyMap {
		sourceVertex, err := g.graph.Vertex(sourceID)
		if err != nil || sourceVertex.URL == "" || sourceVertex.URL == "about:blank" {
			continue
		}
		for targetID := range targetMap {
			targetVertex, err := g.graph.Vertex(targetID)
			if err != nil || targetVertex.URL == "" || targetVertex.URL == "about:blank" {
				continue
			}
			actionLabel := ""
			if targetVertex.NavigationAction != nil {
				actionLabel = targetVertex.NavigationAction.String()
			}
			edges = append(edges, GraphEdge{
				From:   sourceVertex.URL,
				To:     targetVertex.URL,
				Action: actionLabel,
			})
		}
	}
	return nodes, edges
}

// EdgeCount returns the total number of edges in the graph.
func (g *CrawlGraph) EdgeCount() int {
	g.mu.RLock()
	defer g.mu.RUnlock()

	adjacencyMap, err := g.graph.AdjacencyMap()
	if err != nil {
		return 0
	}
	count := 0
	for _, targets := range adjacencyMap {
		count += len(targets)
	}
	return count
}

func (g *CrawlGraph) DrawGraph(file string) error {
	g.mu.RLock()
	defer g.mu.RUnlock()

	f, err := os.Create(file)
	if err != nil {
		return errors.Wrap(err, "could not create graph file")
	}
	defer func() { _ = f.Close() }()

	return draw.DOT(g.graph, f)
}
