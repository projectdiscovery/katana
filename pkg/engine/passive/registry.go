package passive

import (
	"github.com/projectdiscovery/katana/pkg/engine/passive/source"
	"github.com/projectdiscovery/katana/pkg/engine/passive/source/alienvault"
	"github.com/projectdiscovery/katana/pkg/engine/passive/source/commoncrawl"
	"github.com/projectdiscovery/katana/pkg/engine/passive/source/waybackarchive"
)

var Sources = map[string]source.Source{
	"waybackarchive": &waybackarchive.Source{},
	"commoncrawl":    &commoncrawl.Source{},
	"alienvault":     &alienvault.Source{},
}
