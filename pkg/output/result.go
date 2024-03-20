package output

import (
	"time"

	"github.com/projectdiscovery/katana/pkg/navigation"
)

// Result of the crawling
type Result struct {
	Timestamp        time.Time                    `json:"timestamp,omitempty"`
	Request          *navigation.Request          `json:"request,omitempty"`
	Response         *navigation.Response         `json:"response,omitempty"`
	PassiveReference *navigation.PassiveReference `json:"passive,omitempty"`
	Error            string                       `json:"error,omitempty"`
}

// HasResponse checks if the result has a valid response
func (r *Result) HasResponse() bool {
	return r.Response != nil && r.Response.Resp != nil
}
