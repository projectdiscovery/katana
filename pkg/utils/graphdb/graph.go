package graphdb

import (
	"context"

	_ "github.com/mattn/go-sqlite3"
	"github.com/projectdiscovery/katana/ent"
	"github.com/projectdiscovery/katana/ent/endpoint"
)

type GraphDB struct {
	entClient *ent.Client
}

func New() (*GraphDB, error) {
	client, err := ent.Open("sqlite3", "file:ent?mode=memory&cache=shared&_fk=1")
	if err != nil {
		return nil, err
	}

	if err := client.Schema.Create(context.Background()); err != nil {
		return nil, err
	}

	return &GraphDB{entClient: client}, nil
}

func (graphDB *GraphDB) Close() error {
	return graphDB.entClient.Close()
}

func (graphDB *GraphDB) AddEndpointFromURL(ctx context.Context, URL string) (*ent.Endpoint, error) {
	return graphDB.entClient.Endpoint.
		Create().
		SetURL(URL).
		Save(ctx)
}

func (graphDB *GraphDB) ConnectEndpoints(ctx context.Context, source *ent.Endpoint, destinations ...*ent.Endpoint) (int, error) {
	return graphDB.entClient.Endpoint.Update().
		AddLinks(destinations...).
		Save(ctx)
}

func (graphDB *GraphDB) QueryConnections(ctx context.Context, e *ent.Endpoint) ([]*ent.Endpoint, error) {
	return e.QueryLinks().All(ctx)
}

func (graphDB *GraphDB) QueryEndpointWithURL(ctx context.Context, URL string) (*ent.Endpoint, error) {
	return graphDB.entClient.Endpoint.Query().Where(endpoint.URL(URL)).Only(ctx)
}

func (graphDB *GraphDB) GetOrCreateWithURL(ctx context.Context, URL string) (*ent.Endpoint, error) {
	endpoint, err := graphDB.QueryEndpointWithURL(ctx, URL)
	if ent.IsNotFound(err) {
		return graphDB.AddEndpointFromURL(ctx, URL)
	}
	return endpoint, err
}
