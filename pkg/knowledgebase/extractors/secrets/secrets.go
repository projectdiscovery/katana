// Package secrets is a knowledgebase.Extractor that detects exposed
// credentials in crawled response bodies using Praetorian's Titus engine.
// Validation is opt-in because confirming a key sends a live API call to
// the credential's provider, which logs against the credential owner.
package secrets

import (
	"context"
	"net/http"

	titus "github.com/praetorian-inc/titus"
)

const Name = "secrets"

type Config struct {
	Validate bool
}

type Extractor struct {
	scanner *titus.Scanner
}

func New(cfg Config) (*Extractor, error) {
	opts := []titus.Option{titus.WithScoring()}
	if cfg.Validate {
		opts = append(opts, titus.WithValidation())
	}
	s, err := titus.NewScanner(opts...)
	if err != nil {
		return nil, err
	}
	return &Extractor{scanner: s}, nil
}

func (e *Extractor) Name() string { return Name }

func (e *Extractor) Extract(body string, _ *http.Request, _ *http.Response) map[string]any {
	if body == "" {
		return nil
	}
	findings, err := e.scanner.ScanStringWithFindings(context.Background(), body)
	if err != nil || len(findings) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(findings))
	for _, f := range findings {
		if len(f.Matches) == 0 {
			continue
		}
		m := f.Matches[0]
		item := map[string]any{
			"id":   f.ID,
			"rule": m.RuleID,
			"kind": m.RuleName,
		}
		if f.Score != nil {
			item["score"] = f.Score.Final
			item["severity"] = f.Score.SuggestedSeverity
		}
		if m.ValidationResult != nil {
			item["validation"] = map[string]any{
				"status":     string(m.ValidationResult.Status),
				"confidence": m.ValidationResult.Confidence,
			}
		}
		out = append(out, item)
	}
	if len(out) == 0 {
		return nil
	}
	return map[string]any{"findings": out}
}

func (e *Extractor) Close() error {
	if e.scanner == nil {
		return nil
	}
	return e.scanner.Close()
}
