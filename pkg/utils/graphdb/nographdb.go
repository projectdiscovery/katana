package graphdb

import (
	"context"
	"errors"

	_ "github.com/mattn/go-sqlite3"
	"github.com/projectdiscovery/katana/ent"
	"gonum.org/v1/gonum/graph/path"
	"gonum.org/v1/gonum/graph/simple"
)

type NoGraphDB struct {
	undirectedGraph *simple.UndirectedGraph
}

func NewNoGraphDB() (*GraphDB, error) {
	undirectedGraph := simple.NewUndirectedGraph()

	return &GraphDB{undirectedGraph: undirectedGraph}, nil
}

func (noGraphDB *NoGraphDB) Close() error {
	return nil
}

func (noGraphDB *NoGraphDB) AddEndpoint(ctx context.Context, newEndpoint *ent.Endpoint) (*ent.Endpoint, error) {
	noGraphDB.undirectedGraph.AddNode(simple.Node(newEndpoint.ID))
	return newEndpoint, nil
}

func (noGraphDB *NoGraphDB) ConnectEndpoints(ctx context.Context, source *ent.Endpoint, destinations ...*ent.Endpoint) (*ent.Endpoint, error) {
	sourceNode := noGraphDB.undirectedGraph.Node(int64(source.ID))
	if sourceNode == nil {
		return nil, errors.New("source node not found")
	}
	for _, destination := range destinations {
		destinationNode := noGraphDB.undirectedGraph.Node(int64(destination.ID))
		if destinationNode == nil {
			return nil, errors.New("destination node not found")
		}
		noGraphDB.undirectedGraph.SetEdge(noGraphDB.undirectedGraph.NewEdge(sourceNode, destinationNode))
	}

	return nil, nil
}

func (noGraphDB *NoGraphDB) QueryConnections(ctx context.Context, e *ent.Endpoint) ([]*ent.Endpoint, error) {
	var nodes []*ent.Endpoint
	edges := noGraphDB.undirectedGraph.Edges()
	for edges.Next() {
		edge := edges.Edge()
		if edge.From().ID() == int64(e.ID) {
			nodes = append(nodes, &ent.Endpoint{ID: int(edge.To().ID())})
		}
	}
	return nodes, nil
}

func (noGraphDB *NoGraphDB) QueryEndpoints(ctx context.Context) ([]*ent.Endpoint, error) {
	var nodes []*ent.Endpoint
	graphNodes := noGraphDB.undirectedGraph.Nodes()
	for graphNodes.Next() {
		graphNode := graphNodes.Node()
		nodes = append(nodes, &ent.Endpoint{ID: int(graphNode.ID())})
	}
	return nodes, nil
}

func (noGraphDB *NoGraphDB) QueryEndpoint(ctx context.Context, e *ent.Endpoint) (*ent.Endpoint, error) {
	if noGraphDB.undirectedGraph.Node(int64(e.ID)) != nil {
		return e, nil
	}
	return nil, errors.New("node not found")
}

func (noGraphDB *NoGraphDB) HasEndpoint(ctx context.Context, e *ent.Endpoint) (bool, error) {
	return noGraphDB.undirectedGraph.Node(int64(e.ID)) != nil, nil
}

func (noGraphDB *NoGraphDB) GetOrCreate(ctx context.Context, e *ent.Endpoint) (*ent.Endpoint, error) {
	if endpoint, err := noGraphDB.QueryEndpoint(ctx, e); endpoint != nil && err == nil {
		return e, nil
	}
	return noGraphDB.AddEndpoint(ctx, e)
}

func (noGraphDB *NoGraphDB) ShortestPath(ctx context.Context, source, destination *ent.Endpoint) ([]*ent.Endpoint, error) {
	sourceNode, err := lookupGraphNodeById(noGraphDB.undirectedGraph, source.ID)
	if err != nil {
		return nil, err
	}

	dijkstraEngine := path.DijkstraFrom(sourceNode, noGraphDB.undirectedGraph)
	dijkstraPath, _ := dijkstraEngine.To(int64(destination.ID))

	// rebuild the paths in ent.Endpoint
	var nodes []*ent.Endpoint
	for _, dijkstraNode := range dijkstraPath {
		dijkstraNodeId := int(dijkstraNode.ID())
		nodes = append(nodes, &ent.Endpoint{ID: dijkstraNodeId})
	}

	if len(nodes) == 0 {
		return nodes, errors.New("no path found")
	}

	return nodes, nil
}
