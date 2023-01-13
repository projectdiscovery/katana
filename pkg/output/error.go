package output

import "time"

type Error struct {
	Timestamp time.Time `json:"timestamp,omitempty"`
	Endpoint  string    `json:"endpoint,omitempty"`
	Source    string    `json:"source,omitempty"`
	Error     string    `json:"error,omitempty"`
}
