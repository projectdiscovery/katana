package graphdb

import (
	"context"

	"github.com/projectdiscovery/katana/ent"
)

type Graph interface {
	AddEndpoint(ctx context.Context, newEndpoint *ent.Endpoint) (*ent.Endpoint, error)
	ConnectEndpoints(ctx context.Context, source *ent.Endpoint, destinations ...*ent.Endpoint) (*ent.Endpoint, error)
	QueryConnections(ctx context.Context, e *ent.Endpoint) ([]*ent.Endpoint, error)
	QueryEndpoint(ctx context.Context, e *ent.Endpoint) (*ent.Endpoint, error)
	QueryEndpoints(ctx context.Context) ([]*ent.Endpoint, error)
	HasEndpoint(ctx context.Context, e *ent.Endpoint) (bool, error)
	GetOrCreate(ctx context.Context, e *ent.Endpoint) (*ent.Endpoint, error)
	ShortestPath(ctx context.Context, e1, e2 *ent.Endpoint) ([]*ent.Endpoint, error)
	Close() error
}
