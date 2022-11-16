package navigation

import (
	"fmt"
	"os"

	"github.com/dominikbraun/graph"
	"github.com/dominikbraun/graph/draw"
)

type Graph struct {
	graph  graph.Graph[string, State]
	states map[uint64]*State
}

func NewGraph() (*Graph, error) {
	g := &Graph{
		graph:  graph.New(StateHash, graph.Directed()),
		states: make(map[uint64]*State),
	}
	return g, nil
}

func (g *Graph) AddState(req Request, resp Response, name string) (*State, error) {
	newState, err := NewState(req, resp, name)
	if err != nil {
		return nil, err
	}

	// Check if the current state was already visited previously
	// using near approximate search (TODO: current linear complexity => binary search?)
	var existingState *State
	for _, state := range g.states {
		// exact match
		if state.Digest == newState.Digest {
			existingState = state
			break
		}

		// simhash proximity
		similarity := Similarity(newState, state)
		if similarity >= 99 {
			existingState = state
			break
		}
	}
	if existingState == nil {
		g.states[newState.Hash] = newState
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
	} else {
		newState = existingState
	}

	// if req.State is nil => this is a root vertex => nothing to do
	// otherwise we need to create an edge between the previous state and the current one
	if req.State != nil {
		edgeProperties := []func(*graph.EdgeProperties){
			graph.EdgeAttribute("source", req.Source),
			graph.EdgeAttribute("attribute", req.Attribute),
			graph.EdgeAttribute("tag", req.Tag),
			graph.EdgeAttribute("label", fmt.Sprintf("%s\n%s", req.Tag, req.Attribute)),
		}
		if err := g.graph.AddEdge(StateHash(*req.State), StateHash(*newState), edgeProperties...); err != nil {
			return nil, err
		}
	}

	return newState, nil
}

func (g *Graph) ExportTo(outputFile string) error {
	outputGraphFile, err := os.Create(outputFile)
	if err != nil {
		return err
	}
	defer outputGraphFile.Close()

	return draw.DOT(g.graph, outputGraphFile)
}
