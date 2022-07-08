package graphdb

import (
	"context"
	"errors"

	"github.com/projectdiscovery/katana/ent"
	"gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
)

func (graphDB *GraphDB) buildGraph(ctx context.Context) (*simple.UndirectedGraph, error) {
	undirectedGraph := simple.NewUndirectedGraph()
	endpoints, err := graphDB.QueryEndpoints(ctx)
	if err != nil {
		return nil, err
	}
	var nodes []simple.Node
	for _, endpoint := range endpoints {
		node := simple.Node(endpoint.ID)
		nodes = append(nodes, node)
		undirectedGraph.AddNode(node)
	}

	for _, endpoint := range endpoints {
		links, err := graphDB.QueryConnections(ctx, endpoint)
		if err != nil {
			return nil, err
		}
		nodeF, err := lookupNodeById(nodes, endpoint.ID)
		if err != nil {
			return nil, err
		}
		for _, link := range links {
			nodeT, err := lookupNodeById(nodes, link.ID)
			if err != nil {
				return nil, err
			}
			edge := simple.Edge{F: nodeF, T: nodeT}
			undirectedGraph.SetEdge(edge)
		}
	}
	return undirectedGraph, nil
}

func (graphDB *GraphDB) ShortestPath(ctx context.Context, source, destination *ent.Endpoint) ([]*ent.Endpoint, error) {
	if graphDB.undirectedGraph == nil {
		undirectedGraph, err := graphDB.buildGraph(ctx)
		if err != nil {
			return nil, err
		}
		graphDB.undirectedGraph = undirectedGraph
	}

	sourceNode, err := lookupGraphNodeById(graphDB.undirectedGraph, source.ID)
	if err != nil {
		return nil, err
	}

	dijkstraEngine := path.DijkstraFrom(sourceNode, graphDB.undirectedGraph)
	dijkstraPath, _ := dijkstraEngine.To(int64(destination.ID))

	// rebuild the paths in ent.Endpoint
	var nodes []*ent.Endpoint
	for _, dijkstraNode := range dijkstraPath {
		dijkstraNodeId := int(dijkstraNode.ID())
		node, err := graphDB.entClient.Endpoint.Get(ctx, dijkstraNodeId)
		if err != nil {
			return nil, err
		}
		nodes = append(nodes, node)
	}

	if len(nodes) == 0 {
		return nodes, errors.New("no path found")
	}

	return nodes, nil
}

func lookupNodeById(nodes []simple.Node, id int) (simple.Node, error) {
	for _, node := range nodes {
		if node.ID() == int64(id) {
			return node, nil
		}
	}

	return -1, errors.New("node not found")
}

func lookupGraphNodeById(graph graph.Graph, id int) (graph.Node, error) {
	if node := graph.Node(int64(id)); node != nil {
		return node, nil
	}
	return nil, errors.New("node not found")
}
