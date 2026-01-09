// Package graph implements a Directed Graph for storing
// state information during crawling of a Web Application.
package graph

import (
	"os"

	"github.com/dominikbraun/graph"
	"github.com/dominikbraun/graph/draw"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/pkg/engine/headless/types"
)

// CrawlGraph is a graph for storing state information during crawling
type CrawlGraph struct {
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

// AddNavigation adds a navigation to the graph
func (g *CrawlGraph) AddPageState(n types.PageState) error {
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
	pageVertex, err := g.graph.Vertex(id)
	if err != nil {
		return nil, errors.Wrap(err, "could not get vertex")
	}
	return &pageVertex, nil
}

func (g *CrawlGraph) ShortestPath(sourceState, targetState string) ([]*types.Action, error) {
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

func (g *CrawlGraph) DrawGraph(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return errors.Wrap(err, "could not create graph file")
	}
	defer func() { _ = f.Close() }()

	return draw.DOT(g.graph, f)
}
