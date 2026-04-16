package agent

import (
	"context"
	"encoding/json"
	"fmt"
)

// InputSchema describes the JSON Schema for a tool's input parameters.
// Properties maps directly to the Anthropic API's ToolInputSchemaParam.Properties.
// The SDK automatically wraps this with "type": "object".
type InputSchema struct {
	Properties any      // JSON Schema "properties" object (map[string]any)
	Required   []string // Required property names
}

// ToolResultContent represents a single content block in a tool execution result.
// Modeled after the Anthropic API's tool result content union — supports text and images.
type ToolResultContent struct {
	Type   string       // "text" or "image"
	Text   string       // populated when Type == "text"
	Source *ImageSource // populated when Type == "image"
}

// ImageSource is a base64-encoded image for tool results.
type ImageSource struct {
	Type      string // "base64"
	MediaType string // e.g. "image/png", "image/jpeg", "image/webp"
	Data      string // base64-encoded image data
}

// TextContent creates a text tool result content block.
func TextContent(text string) ToolResultContent {
	return ToolResultContent{Type: "text", Text: text}
}

// ImageContent creates a base64 image tool result content block.
func ImageContent(mediaType, base64Data string) ToolResultContent {
	return ToolResultContent{
		Type:   "image",
		Source: &ImageSource{Type: "base64", MediaType: mediaType, Data: base64Data},
	}
}

// Tool is the interface that all agent tools must implement.
type Tool interface {
	// Name returns the tool name used by the model to invoke it.
	Name() string
	// Description returns a human-readable description for the model.
	Description() string
	// Schema returns the JSON Schema for the tool's input parameters.
	Schema() InputSchema
	// Execute runs the tool with the given JSON input and returns content blocks.
	// The input is the raw JSON from the model's tool_use block.
	// Return rich content (text, images) via ToolResultContent blocks.
	// Errors returned here are fed back to the model as tool_result with is_error=true.
	Execute(ctx context.Context, input json.RawMessage) ([]ToolResultContent, error)
}

// DefaultMaxToolResultSize is the default max size (in bytes) for a tool result's text content.
// Results exceeding this are truncated with a notice. This prevents a single tool call from
// blowing the context window. Set MaxResultSize on ToolFunc to override per-tool, or -1 to disable.
const DefaultMaxToolResultSize = 100_000 // ~100KB ≈ ~25k tokens

// ToolFunc wraps a simple text-returning function as a Tool for convenience.
// This follows the http.HandlerFunc pattern. For tools that only return text,
// this avoids having to construct ToolResultContent manually.
type ToolFunc struct {
	ToolName        string
	ToolDescription string
	ToolSchema      InputSchema
	MaxResultSize   int // max bytes for the result text; 0 = use DefaultMaxToolResultSize, -1 = no limit
	Fn              func(ctx context.Context, input json.RawMessage) (string, error)
}

func (t *ToolFunc) Name() string        { return t.ToolName }
func (t *ToolFunc) Description() string { return t.ToolDescription }
func (t *ToolFunc) Schema() InputSchema { return t.ToolSchema }

func (t *ToolFunc) Execute(ctx context.Context, input json.RawMessage) ([]ToolResultContent, error) {
	if t.Fn == nil {
		return nil, fmt.Errorf("tool %q: nil function callback", t.ToolName)
	}
	text, err := t.Fn(ctx, input)
	if err != nil {
		return nil, err
	}
	text = t.truncateResult(text)
	return []ToolResultContent{TextContent(text)}, nil
}

// NewResultTool creates a tool definition for structured output extraction.
// When the model calls this tool, the agent loop (configured with WithResultTool)
// intercepts the call, extracts the specified field from the JSON input, and uses
// it as Result.Response — terminating the loop immediately.
// The Execute method is never called; it exists only to satisfy the Tool interface.
func NewResultTool(name, description, fieldName, fieldDescription string) Tool {
	return &ToolFunc{
		ToolName:        name,
		ToolDescription: description,
		ToolSchema: InputSchema{
			Properties: map[string]any{
				fieldName: map[string]any{
					"type":        "string",
					"description": fieldDescription,
				},
			},
			Required: []string{fieldName},
		},
		Fn: func(_ context.Context, _ json.RawMessage) (string, error) {
			return "", fmt.Errorf("result tool %q was executed directly; should be intercepted by WithResultTool", name)
		},
	}
}

// NewMultiFieldResultTool creates a result tool with multiple required string fields.
// The fields map keys are field names and values are field descriptions.
// Like NewResultTool, the Execute method is never called — the agent loop intercepts
// the call and extracts all fields.
func NewMultiFieldResultTool(name, description string, fields map[string]string) Tool {
	properties := make(map[string]any, len(fields))
	required := make([]string, 0, len(fields))
	for fieldName, fieldDesc := range fields {
		properties[fieldName] = map[string]any{
			"type":        "string",
			"description": fieldDesc,
		}
		required = append(required, fieldName)
	}
	return &ToolFunc{
		ToolName:        name,
		ToolDescription: description,
		ToolSchema: InputSchema{
			Properties: properties,
			Required:   required,
		},
		Fn: func(_ context.Context, _ json.RawMessage) (string, error) {
			return "", fmt.Errorf("result tool %q was executed directly; should be intercepted by WithResultTool", name)
		},
	}
}

// NewFlexResultTool creates a result tool with required and optional string fields.
// requiredFields are mandatory; optionalFields are extracted if present but not enforced.
// Like NewResultTool, the Execute method is never called — the agent loop intercepts
// the call and extracts all fields that are present.
func NewFlexResultTool(name, description string, requiredFields, optionalFields map[string]string) Tool {
	properties := make(map[string]any, len(requiredFields)+len(optionalFields))
	required := make([]string, 0, len(requiredFields))
	for fieldName, fieldDesc := range requiredFields {
		properties[fieldName] = map[string]any{
			"type":        "string",
			"description": fieldDesc,
		}
		required = append(required, fieldName)
	}
	for fieldName, fieldDesc := range optionalFields {
		properties[fieldName] = map[string]any{
			"type":        "string",
			"description": fieldDesc,
		}
	}
	return &ToolFunc{
		ToolName:        name,
		ToolDescription: description,
		ToolSchema: InputSchema{
			Properties: properties,
			Required:   required,
		},
		Fn: func(_ context.Context, _ json.RawMessage) (string, error) {
			return "", fmt.Errorf("result tool %q was executed directly; should be intercepted by WithResultTool", name)
		},
	}
}

// truncateResult truncates the result text if it exceeds the configured max size.
func (t *ToolFunc) truncateResult(text string) string {
	limit := DefaultMaxToolResultSize
	if t.MaxResultSize > 0 {
		limit = t.MaxResultSize
	} else if t.MaxResultSize < 0 {
		return text // no limit
	}
	if len(text) <= limit {
		return text
	}
	return text[:limit] + "\n\n... [truncated, result exceeded size limit]"
}
