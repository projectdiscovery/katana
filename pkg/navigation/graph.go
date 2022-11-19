package navigation

import (
	"fmt"

	"github.com/dominikbraun/graph"
)

type GraphOption func(g *Graph) error

func WithApproximation(g *Graph) error {
	g.Approximate = true
	return nil
}

type Graph struct {
	graph       graph.Graph[string, State]
	data        *GraphData
	Approximate bool
}

func NewGraph(graphOptions ...GraphOption) (*Graph, error) {
	g := &Graph{
		graph: graph.New(StateHash, graph.Directed()),
		data:  &GraphData{},
	}

	for _, graphOption := range graphOptions {
		if err := graphOption(g); err != nil {
			return nil, err
		}
	}

	return g, nil
}

func (g *Graph) AddState(req Request, resp Response, name string) (*State, error) {
	newState, err := g.nearApproximateOrNew(req, resp, name)
	if err != nil {
		return nil, err
	}

	g.data.Vertexes = append(g.data.Vertexes, newState)
	// Color edge
	// Html State => Green
	// Static File => Red
	var color string
	if ContentTypeIsTextHtml(resp.Resp.Header, resp.Body) {
		color = "green"
	} else {
		color = "red"
	}
	if err := g.graph.AddVertex(*newState, graph.VertexAttribute("color", color)); err != nil {
		return nil, err
	}

	// if req.State is nil => this is a root vertex => nothing to do
	// otherwise we need to create an edge between the previous state and the current one
	if req.State != nil {
		properties := make(map[string]string)
		properties["source"] = req.Source
		properties["attribute"] = req.Attribute
		properties["tag"] = req.Source
		properties["source"] = req.Tag
		properties["label"] = fmt.Sprintf("%s\n%s", req.Tag, req.Attribute)
		edgeProperties := g.toEdgeProperties(properties)
		if err := g.graph.AddEdge(StateHash(*req.State), StateHash(*newState), edgeProperties...); err != nil {
			return nil, err
		}
		g.data.Edges = append(g.data.Edges, Edge{
			From:       req.State,
			To:         newState,
			Properties: properties,
		})
	}

	return newState, nil
}

func (g *Graph) toEdgeProperties(properties map[string]string) (edgeProperties []func(*graph.EdgeProperties)) {
	for key, value := range properties {
		edgeProperties = append(edgeProperties, graph.EdgeAttribute(key, value))
	}
	return
}

func (g *Graph) nearApproximateOrNew(req Request, resp Response, name string) (*State, error) {
	newState, err := NewState(req, resp, name)
	if err != nil {
		return nil, err
	}

	if !g.Approximate {
		return newState, nil
	}

	// Check if the current state was already visited previously
	// using near approximate search (TODO: current linear complexity => binary search?)
	var existingState *State
	for _, state := range g.data.Vertexes {
		// exact match
		if state.Digest == newState.Digest {
			return existingState, nil
		}

		// simhash proximity
		similarity := Similarity(newState, state)
		if similarity >= 94 {
			return existingState, nil
		}
	}

	return newState, nil
}
