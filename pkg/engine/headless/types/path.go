package types

import (
	"encoding/json"
)

// Path is an ordered sequence of actions from the crawl entrypoint to a location.
// It is the unit Burp-style cartography rewalks and audits replay.
type Path struct {
	EntryID   string    `json:"entry_id"`
	TargetID  string    `json:"target_id"`
	TargetURL string    `json:"target_url,omitempty"`
	Steps     []*Action `json:"steps,omitempty"`
}

// Len returns the number of navigation steps.
func (p *Path) Len() int {
	if p == nil {
		return 0
	}
	return len(p.Steps)
}

// Clone returns a shallow copy with a new Steps slice (actions are shared).
func (p *Path) Clone() *Path {
	if p == nil {
		return nil
	}
	out := &Path{
		EntryID:   p.EntryID,
		TargetID:  p.TargetID,
		TargetURL: p.TargetURL,
	}
	if len(p.Steps) > 0 {
		out.Steps = make([]*Action, len(p.Steps))
		copy(out.Steps, p.Steps)
	}
	return out
}

// MarshalJSON ensures nil Steps serialize as [] not null.
func (p *Path) MarshalJSON() ([]byte, error) {
	steps := p.Steps
	if steps == nil {
		steps = []*Action{}
	}
	return json.Marshal(struct {
		EntryID   string    `json:"entry_id"`
		TargetID  string    `json:"target_id"`
		TargetURL string    `json:"target_url,omitempty"`
		Steps     []*Action `json:"steps"`
	}{
		EntryID:   p.EntryID,
		TargetID:  p.TargetID,
		TargetURL: p.TargetURL,
		Steps:     steps,
	})
}
