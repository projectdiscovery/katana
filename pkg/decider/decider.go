// Package decider asks a language model typed questions at points where the
// crawler would otherwise guess. Answers are validated against what the crawler
// offers, so the model can never introduce values or actions of its own.
package decider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
)

// Kind is the shape of the answer a question expects.
type Kind string

const (
	Bool   Kind = "bool"
	Choice Kind = "choice"
	Score  Kind = "score"
)

// Question is a single typed question about a page.
type Question struct {
	Kind   Kind   `json:"kind"`
	Prompt string `json:"prompt"`
	// Options are the only valid answers for a Choice question.
	Options []string `json:"options,omitempty"`
	// Context is page content, which is untrusted input.
	Context string `json:"context,omitempty"`
}

// Answer holds the field matching the question kind. Score is in [0, 1].
type Answer struct {
	Bool   bool    `json:"bool,omitempty"`
	Choice string  `json:"choice,omitempty"`
	Score  float64 `json:"score,omitempty"`
}

// Decider answers typed questions.
type Decider interface {
	Decide(ctx context.Context, q Question) (Answer, error)
}

var (
	ErrNotRecorded    = errors.New("decider: answer not recorded")
	ErrBudgetExceeded = errors.New("decider: call budget exceeded")
	// ErrUnavailable means the provider rejected the url, model or key, so retrying cannot help.
	ErrUnavailable = errors.New("decider: provider unavailable")
)

func (q Question) validate() error {
	switch q.Kind {
	case Bool, Score:
		return nil
	case Choice:
		if len(q.Options) == 0 {
			return errors.New("decider: choice question without options")
		}
		return nil
	default:
		return fmt.Errorf("decider: unknown question kind %q", q.Kind)
	}
}

func (q Question) check(a Answer) error {
	switch q.Kind {
	case Choice:
		if !slices.Contains(q.Options, a.Choice) {
			return fmt.Errorf("decider: answer %q is not an option", a.Choice)
		}
	case Score:
		if a.Score < 0 || a.Score > 1 {
			return fmt.Errorf("decider: score %v out of range", a.Score)
		}
	}
	return nil
}

func (q Question) key() string {
	data, _ := json.Marshal(q)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
