package types

// Path is an ordered sequence of actions from the crawl entrypoint to a location.
// It is the unit Burp-style cartography rewalks and audits replay.
type Path struct {
	EntryID   string    `json:"entry_id"`
	TargetID  string    `json:"target_id"`
	TargetURL string    `json:"target_url,omitempty"`
	Steps     []*Action `json:"steps"`
}
