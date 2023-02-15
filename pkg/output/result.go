package output

import (
	"time"

	"github.com/projectdiscovery/katana/pkg/navigation"
)

// Result of the crawling
type Result struct {
	Timestamp time.Time           `json:"timestamp,omitempty"`
	Request   navigation.Request  `json:"request,omitempty"`
	Response  navigation.Response `json:"response,omitempty"`
}
