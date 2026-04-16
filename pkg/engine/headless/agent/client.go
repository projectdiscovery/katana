package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"net"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Model constants for convenience.
const (
	ModelOpus4_6   = string(anthropic.ModelClaudeOpus4_6)
	ModelSonnet4_6 = string(anthropic.ModelClaudeSonnet4_6)
	ModelHaiku4_5  = string(anthropic.ModelClaudeHaiku4_5)
)

// ContentBlock represents a single block in the model's response.
// It can be either text or a tool use request.
type ContentBlock struct {
	Type  string          // "text" or "tool_use"
	Text  string          // populated when Type == "text"
	ID    string          // tool use ID, populated when Type == "tool_use"
	Name  string          // tool name, populated when Type == "tool_use"
	Input json.RawMessage // tool input JSON, populated when Type == "tool_use"
}

// Response is the model's response from a single API call.
type Response struct {
	Content    []ContentBlock
	StopReason string // "end_turn", "tool_use", "max_tokens"
	Usage      TokenUsage
	RetryCount int   // number of retries before this response succeeded
	RetryWaitMS int64 // total time spent waiting on retries (milliseconds)
}

// TokenUsage tracks token consumption.
type TokenUsage struct {
	InputTokens              int64
	OutputTokens             int64
	CacheCreationInputTokens int64
	CacheReadInputTokens     int64
}

// Message is a single message in the conversation history.
type Message struct {
	Role        string         // "user" or "assistant"
	Text        string         // for simple user text messages
	Content     []ContentBlock // for assistant messages (text + tool_use blocks)
	ToolResults []ToolResult   // for user messages carrying tool results
}

// ToolResult pairs a tool use ID with its execution output.
// Content supports rich results (text, images) matching the Anthropic API's
// tool_result content union.
type ToolResult struct {
	ToolUseID string
	Content   []ToolResultContent
	IsError   bool
}

// ClientRequest contains all parameters for an agent API call.
type ClientRequest struct {
	System          string
	Messages        []Message
	Tools           []Tool
	MaxTokens       int64
	ForceToolChoice string // when non-empty, forces the model to call this specific tool
}

// AgentClient is the interface for LLM calls that support the agent tool-calling loop.
type AgentClient interface {
	Send(ctx context.Context, req *ClientRequest) (*Response, error)
}

// AnthropicAgentClient implements AgentClient using the Anthropic SDK.
type AnthropicAgentClient struct {
	client        *anthropic.Client
	model         anthropic.Model
	maxRetries    int
	retryBaseWait time.Duration
	logger        *slog.Logger
}

// AnthropicAgentOption configures the Anthropic agent client.
type AnthropicAgentOption func(*anthropicAgentConfig)

const (
	defaultMaxRetries    = 5
	defaultRetryBaseWait = 2 * time.Second
)

type anthropicAgentConfig struct {
	model         anthropic.Model
	baseURL       string
	maxRetries    int
	retryBaseWait time.Duration
	logger        *slog.Logger
}

// WithModel sets the model. Default: claude-sonnet-4-6.
func WithModel(model string) AnthropicAgentOption {
	return func(c *anthropicAgentConfig) { c.model = anthropic.Model(model) }
}

// WithBaseURL sets a custom base URL for the Anthropic API.
func WithBaseURL(url string) AnthropicAgentOption {
	return func(c *anthropicAgentConfig) { c.baseURL = url }
}

// WithMaxRetries sets the max retries on transient errors (default: 5).
// Set to 0 to disable retries. Negative values are clamped to 0.
func WithMaxRetries(n int) AnthropicAgentOption {
	return func(c *anthropicAgentConfig) {
		if n < 0 {
			n = 0
		}
		c.maxRetries = n
	}
}

// WithClientLogger sets the structured logger for the Anthropic client (retry logging).
func WithClientLogger(l *slog.Logger) AnthropicAgentOption {
	return func(c *anthropicAgentConfig) { c.logger = l }
}

// NewAnthropicAgentClient creates an Anthropic client for agent tool-calling loops.
func NewAnthropicAgentClient(apiKey string, opts ...AnthropicAgentOption) *AnthropicAgentClient {
	cfg := &anthropicAgentConfig{
		model:         anthropic.ModelClaudeSonnet4_6,
		maxRetries:    defaultMaxRetries,
		retryBaseWait: defaultRetryBaseWait,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	clientOpts := []option.RequestOption{
		option.WithAPIKey(apiKey),
	}
	if cfg.baseURL != "" {
		clientOpts = append(clientOpts, option.WithBaseURL(cfg.baseURL))
	}

	client := anthropic.NewClient(clientOpts...)
	logger := cfg.logger
	if logger == nil {
		logger = slog.Default()
	}
	return &AnthropicAgentClient{
		client:        &client,
		model:         cfg.model,
		maxRetries:    cfg.maxRetries,
		retryBaseWait: cfg.retryBaseWait,
		logger:        logger,
	}
}

// Send implements AgentClient.
func (c *AnthropicAgentClient) Send(ctx context.Context, req *ClientRequest) (*Response, error) {
	if req == nil {
		return nil, fmt.Errorf("anthropic: nil client request")
	}
	// Convert tools to Anthropic ToolUnionParam.
	// Set cache_control on the last tool so the system prompt + all tool
	// definitions are cached together (Anthropic caches up to the last
	// cache_control breakpoint).
	var tools []anthropic.ToolUnionParam
	for i, t := range req.Tools {
		schema := t.Schema()
		tp := anthropic.ToolParam{
			Name:        t.Name(),
			Description: anthropic.String(t.Description()),
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: schema.Properties,
				Required:   schema.Required,
			},
		}
		if i == len(req.Tools)-1 {
			tp.CacheControl = anthropic.NewCacheControlEphemeralParam()
		}
		tools = append(tools, anthropic.ToolUnionParam{OfTool: &tp})
	}

	// Convert messages to Anthropic MessageParam
	var apiMessages []anthropic.MessageParam
	for _, m := range req.Messages {
		switch m.Role {
		case "user":
			if len(m.ToolResults) > 0 {
				var blocks []anthropic.ContentBlockParamUnion
				for _, tr := range m.ToolResults {
					blocks = append(blocks, toolResultToSDK(tr))
				}
				apiMessages = append(apiMessages, anthropic.NewUserMessage(blocks...))
			} else {
				apiMessages = append(apiMessages, anthropic.NewUserMessage(anthropic.NewTextBlock(m.Text)))
			}
		case "assistant":
			var blocks []anthropic.ContentBlockParamUnion
			for _, cb := range m.Content {
				switch cb.Type {
				case "text":
					blocks = append(blocks, anthropic.NewTextBlock(cb.Text))
				case "tool_use":
					blocks = append(blocks, anthropic.NewToolUseBlock(cb.ID, cb.Input, cb.Name))
				}
			}
			apiMessages = append(apiMessages, anthropic.NewAssistantMessage(blocks...))
		}
	}

	// Add a cache breakpoint on the conversation history so that prior turns
	// are cached across API calls. The system prompt + tools may be under the
	// 1024-token minimum for caching, but system + tools + prior turns will
	// exceed it on turn 2+. We mark the second-to-last message (the stable
	// "previous turn" boundary) so the growing conversation prefix is cached.
	if len(apiMessages) >= 3 {
		prev := &apiMessages[len(apiMessages)-2]
		if n := len(prev.Content); n > 0 {
			last := &prev.Content[n-1]
			cc := anthropic.NewCacheControlEphemeralParam()
			switch {
			case last.OfToolResult != nil:
				last.OfToolResult.CacheControl = cc
			case last.OfToolUse != nil:
				last.OfToolUse.CacheControl = cc
			case last.OfText != nil:
				last.OfText.CacheControl = cc
			}
		}
	}

	maxTokens := req.MaxTokens
	if maxTokens == 0 {
		maxTokens = 16384
	}

	params := anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: maxTokens,
		Messages:  apiMessages,
		Tools:     tools,
	}

	if req.System != "" {
		params.System = []anthropic.TextBlockParam{
			{
				Text:         req.System,
				CacheControl: anthropic.NewCacheControlEphemeralParam(),
			},
		}
	}

	if req.ForceToolChoice != "" {
		params.ToolChoice = anthropic.ToolChoiceUnionParam{
			OfTool: &anthropic.ToolChoiceToolParam{
				Name: req.ForceToolChoice,
			},
		}
	}

	// Call API with retry on transient errors (429, 500, 529)
	resp, retryCount, retryWaitMS, err := c.sendWithRetry(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	// Convert response
	result := &Response{
		StopReason: string(resp.StopReason),
		Usage: TokenUsage{
			InputTokens:              resp.Usage.InputTokens,
			OutputTokens:             resp.Usage.OutputTokens,
			CacheCreationInputTokens: resp.Usage.CacheCreationInputTokens,
			CacheReadInputTokens:     resp.Usage.CacheReadInputTokens,
		},
		RetryCount:  retryCount,
		RetryWaitMS: retryWaitMS,
	}

	for _, block := range resp.Content {
		switch block.Type {
		case "text":
			result.Content = append(result.Content, ContentBlock{
				Type: "text",
				Text: block.Text,
			})
		case "tool_use":
			toolUse := block.AsToolUse()
			inputBytes, err := json.Marshal(toolUse.Input)
			if err != nil {
				return nil, fmt.Errorf("failed to marshal tool input: %w", err)
			}
			result.Content = append(result.Content, ContentBlock{
				Type:  "tool_use",
				ID:    toolUse.ID,
				Name:  toolUse.Name,
				Input: inputBytes,
			})
		}
	}

	return result, nil
}

// sendWithRetry calls the Anthropic API with exponential backoff on transient errors.
// It uses streaming to avoid the 10-minute timeout on non-streaming requests.
// Retries on both HTTP errors (429, 500, 529) and network/connection errors.
// Returns the response plus retry metrics (count and total wait time in ms).
func (c *AnthropicAgentClient) sendWithRetry(ctx context.Context, params anthropic.MessageNewParams) (msg *anthropic.Message, retryCount int, retryWaitMS int64, err error) {
	var lastErr error
	for attempt := range c.maxRetries + 1 {
		resp, apiErr := c.streamToMessage(ctx, params)
		if apiErr == nil {
			return resp, retryCount, retryWaitMS, nil
		}

		if isRetryableError(apiErr) && attempt < c.maxRetries {
			wait := c.retryBaseWait * time.Duration(math.Pow(2, float64(attempt)))
			retryCount++
			retryWaitMS += wait.Milliseconds()
			c.logger.Warn("retrying API call",
				"attempt", attempt+1,
				"max_retries", c.maxRetries,
				"wait_ms", wait.Milliseconds(),
				"error", apiErr,
			)
			select {
			case <-ctx.Done():
				return nil, retryCount, retryWaitMS, ctx.Err()
			case <-time.After(wait):
			}
			lastErr = apiErr
			continue
		}

		return nil, retryCount, retryWaitMS, apiErr
	}
	return nil, retryCount, retryWaitMS, lastErr
}

// streamToMessage uses the streaming API and accumulates the result into a
// complete Message. This avoids the Anthropic API error "streaming is required
// for operations that may take longer than 10 minutes".
func (c *AnthropicAgentClient) streamToMessage(ctx context.Context, params anthropic.MessageNewParams) (*anthropic.Message, error) {
	stream := c.client.Messages.NewStreaming(ctx, params)
	message := anthropic.Message{}
	for stream.Next() {
		event := stream.Current()
		if err := message.Accumulate(event); err != nil {
			return nil, fmt.Errorf("stream accumulate error: %w", err)
		}
	}
	if err := stream.Err(); err != nil {
		return nil, err
	}
	return &message, nil
}

func isRetryableStatus(status int) bool {
	return status == 429 || status == 500 || status == 529
}

// isRetryableError returns true if the error is transient and worth retrying.
// This covers both HTTP status errors (429, 500, 529) and network/connection errors
// like TCP resets, DNS failures, and route unreachable.
func isRetryableError(err error) bool {
	// HTTP status code errors
	var apiErr *anthropic.Error
	if errors.As(err, &apiErr) {
		return isRetryableStatus(apiErr.StatusCode)
	}

	// Network errors (TCP reset, connection refused, DNS failures, etc.)
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}

	// Catch connection errors that may not implement net.Error
	errStr := err.Error()
	return strings.Contains(errStr, "connection refused") ||
		strings.Contains(errStr, "no route to host") ||
		strings.Contains(errStr, "connection reset") ||
		strings.Contains(errStr, "broken pipe") ||
		strings.Contains(errStr, "i/o timeout") ||
		strings.Contains(errStr, "EOF")
}

// toolResultToSDK converts our ToolResult (with rich content blocks) into
// the Anthropic SDK's ContentBlockParamUnion for tool results.
func toolResultToSDK(tr ToolResult) anthropic.ContentBlockParamUnion {
	var content []anthropic.ToolResultBlockParamContentUnion
	for _, c := range tr.Content {
		switch c.Type {
		case "text":
			content = append(content, anthropic.ToolResultBlockParamContentUnion{
				OfText: &anthropic.TextBlockParam{Text: c.Text},
			})
		case "image":
			if c.Source != nil {
				content = append(content, anthropic.ToolResultBlockParamContentUnion{
					OfImage: &anthropic.ImageBlockParam{
						Source: anthropic.ImageBlockParamSourceUnion{
							OfBase64: &anthropic.Base64ImageSourceParam{
								Data:      c.Source.Data,
								MediaType: anthropic.Base64ImageSourceMediaType(c.Source.MediaType),
							},
						},
					},
				})
			}
		}
	}
	return anthropic.ContentBlockParamUnion{
		OfToolResult: &anthropic.ToolResultBlockParam{
			ToolUseID: tr.ToolUseID,
			Content:   content,
			IsError:   anthropic.Bool(tr.IsError),
		},
	}
}
