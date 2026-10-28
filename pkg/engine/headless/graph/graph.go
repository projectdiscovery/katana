// Package graph implements a Directed Graph for storing
// state information during crawling of a Web Application.
package graph

import (
	"os"
	"slices"
	"sort"

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
			ep.Data = n.NavigationAction
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
		ep.Data = action
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

// PathsFrom returns a replayable path from entryID to every other vertex it
// reaches. One breadth-first walk builds all paths, so each path has the fewest
// steps. Each step is the action recorded on the traversed edge; an edge
// without an action is not followed. Vertices the walk cannot reach are returned, sorted, in unreachable so
// callers can report them.
func (g *CrawlGraph) PathsFrom(entryID string) (paths []*types.Path, unreachable []string, err error) {
	adjacency, err := g.graph.AdjacencyMap()
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not get adjacency map")
	}
	if _, ok := adjacency[entryID]; !ok {
		return nil, nil, errors.Errorf("entry state %s is not in the crawl graph", entryID)
	}

	steps := map[string][]*types.Action{entryID: {}}
	queue := []string{entryID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]

		targets := make([]string, 0, len(adjacency[id]))
		for target := range adjacency[id] {
			targets = append(targets, target)
		}
		sort.Strings(targets)

		for _, target := range targets {
			if _, seen := steps[target]; seen {
				continue
			}
			// The step is the action on the traversed edge. The target's own
			// NavigationAction can start from a different origin state.
			action, _ := adjacency[id][target].Properties.Data.(*types.Action)
			if action == nil {
				continue
			}
			state, err := g.GetPageState(target)
			if err != nil {
				return nil, nil, err
			}
			targetSteps := steps[id]
			if state.URL != "about:blank" {
				// Clip forces a copy, so sibling paths never share a backing array.
				targetSteps = append(slices.Clip(targetSteps), action)
			}
			steps[target] = targetSteps
			queue = append(queue, target)

			if state.URL == "about:blank" {
				continue
			}
			paths = append(paths, &types.Path{
				EntryID:   entryID,
				TargetID:  target,
				TargetURL: state.URL,
				Steps:     targetSteps,
			})
		}
	}

	for id := range adjacency {
		if _, ok := steps[id]; !ok {
			unreachable = append(unreachable, id)
		}
	}
	sort.Strings(unreachable)
	return paths, unreachable, nil
}

func (g *CrawlGraph) DrawGraph(file string) error {
	f, err := os.Create(file)
	if err != nil {
		return errors.Wrap(err, "could not create graph file")
	}
	defer func() { _ = f.Close() }()

	return draw.DOT(g.graph, f)
}
