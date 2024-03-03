package source

import (
	"context"

	"github.com/projectdiscovery/katana/pkg/engine/common"
)

type Source interface {
	Run(context.Context, *common.Shared, string) <-chan Result
	Name() string
	NeedsKey() bool
	AddApiKeys([]string)
}

type Result struct {
	Source    string
	Value     string
	Reference string
	Error     error
}
