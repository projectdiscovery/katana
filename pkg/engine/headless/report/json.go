package report

import (
	"encoding/json"
	"fmt"
	"io"
)

// RenderJSON writes the report as pretty-printed JSON.
func RenderJSON(w io.Writer, report *SiteReport) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(report); err != nil {
		return fmt.Errorf("render json: %w", err)
	}
	return nil
}

// RenderJSONCompact writes the report as single-line JSON.
func RenderJSONCompact(w io.Writer, report *SiteReport) error {
	if err := json.NewEncoder(w).Encode(report); err != nil {
		return fmt.Errorf("render json compact: %w", err)
	}
	return nil
}
