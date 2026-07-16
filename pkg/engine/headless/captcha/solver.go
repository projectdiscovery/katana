package captcha

import (
	"context"
	"fmt"
)

type Solution struct {
	Token    string
	Provider Provider
}

type Solver interface {
	Solve(ctx context.Context, info *Info) (*Solution, error)
}

func NewSolver(provider, apiKey string) (Solver, error) {
	constructor, ok := solverRegistry[provider]
	if !ok {
		return nil, fmt.Errorf("unsupported captcha solver provider: %s", provider)
	}
	return constructor(apiKey)
}

type SolverConstructor func(apiKey string) (Solver, error)

var solverRegistry = map[string]SolverConstructor{}

func RegisterSolver(name string, constructor SolverConstructor) {
	solverRegistry[name] = constructor
}
