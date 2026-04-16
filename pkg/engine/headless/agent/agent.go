package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

const (
	DefaultMaxTurns    = 25
	DefaultMaxTokens   = 16384 // per API call
	DefaultTokenBudget = 0     // 0 = unlimited
)

// Sentinel errors for agent termination conditions.
var (
	ErrMaxTurnsReached     = fmt.Errorf("agent: max turns reached")
	ErrTokenBudgetExceeded = fmt.Errorf("agent: token budget exceeded")
)

// TurnEvent is passed to the OnTurn callback after each API round-trip.
type TurnEvent struct {
	Turn       int
	Response   *Response
	TotalUsage TokenUsage
}

// ToolCallEvent is passed to the OnToolCall callback after each tool execution.
type ToolCallEvent struct {
	Turn    int
	Name    string          // tool name
	Input   json.RawMessage // raw JSON input the model sent
	Output  []ToolResultContent
	IsError bool
}

type agentConfig struct {
	systemPrompt              string
	tools                     []Tool
	maxTurns                  int
	maxTokens                 int64    // per API call
	tokenBudget               int64    // cumulative limit, 0 = unlimited
	maxParallelTools          int      // max concurrent tool executions, 0 = unlimited
	resultToolName            string   // if set, intercept this tool call as the final structured result
	resultFieldName           string   // JSON field in the tool's input to extract as Response
	resultExtraFields         []string // additional field names to extract from result tool into Result.Metadata
	minToolCallsBeforeResult  int      // minimum non-result tool calls before accepting the result tool
	resultGateRejectionPrompt string   // custom rejection message when gate blocks early submission
	onTurn                    func(TurnEvent)
	onToolCall                func(ToolCallEvent)
	logger                    *slog.Logger
}

func defaultConfig() *agentConfig {
	return &agentConfig{
		maxTurns:  DefaultMaxTurns,
		maxTokens: DefaultMaxTokens,
		logger:    slog.Default(),
	}
}

// AgentOption is a functional option for configuring an Agent.
type AgentOption func(*agentConfig)

// WithSystemPrompt sets the agent's system prompt.
func WithSystemPrompt(prompt string) AgentOption {
	return func(c *agentConfig) { c.systemPrompt = prompt }
}

// WithTools adds tools to the agent's tool set.
func WithTools(tools ...Tool) AgentOption {
	return func(c *agentConfig) { c.tools = append(c.tools, tools...) }
}

// WithMaxTurns sets the maximum number of API round-trips (default: 25).
func WithMaxTurns(n int) AgentOption {
	return func(c *agentConfig) { c.maxTurns = n }
}

// WithMaxTokensPerCall sets the max tokens per API call (default: 16384).
func WithMaxTokensPerCall(n int64) AgentOption {
	return func(c *agentConfig) { c.maxTokens = n }
}

// WithTokenBudget sets the cumulative token budget across all turns. 0 = unlimited.
func WithTokenBudget(n int64) AgentOption {
	return func(c *agentConfig) { c.tokenBudget = n }
}

// WithMaxParallelTools sets the max concurrent tool executions per turn (default: unlimited).
// Use this to prevent rate limit issues when tools make external API calls (e.g. sub-agent spawns).
func WithMaxParallelTools(n int) AgentOption {
	return func(c *agentConfig) { c.maxParallelTools = n }
}

// WithResultTool configures a "result tool" — a special tool that, when called by the
// model, has its input extracted as the agent's final Response and terminates the loop.
// This is used to force the model to return structured output (e.g. documentation)
// through a tool call, cleanly separating it from any preamble/thinking text.
// The tool must also be registered via WithTools so the API knows about it.
//
// Optional extraFields specify additional field names to extract from the tool's input
// into Result.Metadata (e.g. "audit_context"). The primary fieldName goes into Response.
func WithResultTool(toolName, fieldName string, extraFields ...string) AgentOption {
	return func(c *agentConfig) {
		c.resultToolName = toolName
		c.resultFieldName = fieldName
		c.resultExtraFields = extraFields
	}
}

// WithMinToolCallsBeforeResult sets the minimum number of non-result tool calls
// the agent must make before the result tool submission is accepted. If the agent
// tries to submit early, the submission is rejected with a nudge to keep exploring.
// An optional rejectionPrompt overrides the default rejection message.
func WithMinToolCallsBeforeResult(n int, rejectionPrompt string) AgentOption {
	return func(c *agentConfig) {
		c.minToolCallsBeforeResult = n
		c.resultGateRejectionPrompt = rejectionPrompt
	}
}

// WithOnTurn registers a callback invoked after each API round-trip.
// Use this to show progress, log turns, or implement custom stopping logic.
func WithOnTurn(fn func(TurnEvent)) AgentOption {
	return func(c *agentConfig) { c.onTurn = fn }
}

// WithOnToolCall registers a callback invoked after each tool execution completes.
// Called concurrently when tools run in parallel — the callback must be safe for concurrent use.
func WithOnToolCall(fn func(ToolCallEvent)) AgentOption {
	return func(c *agentConfig) { c.onToolCall = fn }
}

// WithAgentLogger sets the structured logger for the agent.
// A nil logger falls back to slog.Default().
func WithAgentLogger(l *slog.Logger) AgentOption {
	return func(c *agentConfig) {
		if l == nil {
			l = slog.Default()
		}
		c.logger = l
	}
}

// Result is the output of an agent run.
type Result struct {
	// Response is the final text output from the agent.
	Response string
	// Metadata holds extra fields extracted from a multi-field result tool.
	// For example, when WithResultTool is configured with extra fields like "audit_context",
	// their values are extracted here keyed by field name.
	Metadata map[string]string
	// Messages is the full conversation history (user, assistant, tool results).
	// Useful for debugging, tracing, or feeding intermediate findings into downstream pipeline steps.
	Messages []Message
	// TotalUsage is the cumulative token usage across all turns.
	TotalUsage TokenUsage
	// Turns is the number of API round-trips made.
	Turns int
	// TotalRetryCount is the cumulative number of retries across all turns.
	TotalRetryCount int
	// TotalRetryWaitMS is the cumulative time spent waiting on retries (milliseconds).
	TotalRetryWaitMS int64
}

// Agent is an LLM agent that runs a tool-calling loop.
type Agent struct {
	client AgentClient
	config *agentConfig
}

// New creates a new Agent with the given client and options.
func New(client AgentClient, opts ...AgentOption) *Agent {
	cfg := defaultConfig()
	for _, opt := range opts {
		opt(cfg)
	}
	return &Agent{
		client: client,
		config: cfg,
	}
}

// Run starts the agent with the given user message and runs the tool-calling loop
// until the model produces a final text response, max turns is reached, or the
// token budget is exceeded.
func (a *Agent) Run(ctx context.Context, userMessage string) (*Result, error) {
	toolMap := make(map[string]Tool, len(a.config.tools))
	for _, t := range a.config.tools {
		if t == nil {
			continue
		}
		toolMap[t.Name()] = t
	}

	messages := []Message{
		{Role: "user", Text: userMessage},
	}

	var totalUsage TokenUsage
	var totalRetryCount int
	var totalRetryWaitMS int64
	var totalToolCalls atomic.Int64
	turns := 0

	for turns < a.config.maxTurns {
		if err := ctx.Err(); err != nil {
			return nil, fmt.Errorf("agent cancelled: %w", err)
		}

		turns++
		a.config.logger.Info("agent sending API request",
			"turn", turns,
			"max_turns", a.config.maxTurns,
			"messages", len(messages),
			"total_input_tokens", totalUsage.InputTokens,
			"total_output_tokens", totalUsage.OutputTokens,
		)

		resp, err := a.client.Send(ctx, &ClientRequest{
			System:    a.config.systemPrompt,
			Messages:  messages,
			Tools:     a.config.tools,
			MaxTokens: a.config.maxTokens,
		})
		if err != nil {
			return nil, fmt.Errorf("turn %d: %w", turns, err)
		}

		totalUsage.InputTokens += resp.Usage.InputTokens
		totalUsage.OutputTokens += resp.Usage.OutputTokens
		totalUsage.CacheCreationInputTokens += resp.Usage.CacheCreationInputTokens
		totalUsage.CacheReadInputTokens += resp.Usage.CacheReadInputTokens
		totalRetryCount += resp.RetryCount
		totalRetryWaitMS += resp.RetryWaitMS

		if a.config.onTurn != nil {
			a.config.onTurn(TurnEvent{
				Turn:       turns,
				Response:   resp,
				TotalUsage: totalUsage,
			})
		}

		messages = append(messages, Message{
			Role:    "assistant",
			Content: resp.Content,
		})

		if a.config.tokenBudget > 0 {
			total := totalUsage.InputTokens + totalUsage.OutputTokens
			if total > a.config.tokenBudget {
				a.config.logger.Warn("token budget exceeded",
					"total", total,
					"budget", a.config.tokenBudget,
				)
				return &Result{
					Response:         extractText(resp.Content),
					Messages:         messages,
					TotalUsage:       totalUsage,
					Turns:            turns,
					TotalRetryCount:  totalRetryCount,
					TotalRetryWaitMS: totalRetryWaitMS,
				}, ErrTokenBudgetExceeded
			}
		}

		if resp.StopReason != "tool_use" {
			text := extractText(resp.Content)
			a.config.logger.Info("agent complete",
				"turns", turns,
				"stop_reason", resp.StopReason,
				"response_len", len(text),
				"total_input_tokens", totalUsage.InputTokens,
				"total_output_tokens", totalUsage.OutputTokens,
			)
			return &Result{
				Response:         text,
				Messages:         messages,
				TotalUsage:       totalUsage,
				Turns:            turns,
				TotalRetryCount:  totalRetryCount,
				TotalRetryWaitMS: totalRetryWaitMS,
			}, nil
		}

		// Collect tool use blocks
		var toolUseBlocks []ContentBlock
		for _, block := range resp.Content {
			if block.Type == "tool_use" {
				toolUseBlocks = append(toolUseBlocks, block)
			}
		}

		// Check for result tool — if the model called it, extract the content
		// and return immediately without executing any tools.
		// If minToolCallsBeforeResult is set and the agent hasn't explored enough,
		// reject the submission and let the loop continue.
		if a.config.resultToolName != "" {
			for idx, block := range toolUseBlocks {
				if block.Name != a.config.resultToolName {
					continue
				}

				callsSoFar := int(totalToolCalls.Load())
				if a.config.minToolCallsBeforeResult > 0 && callsSoFar < a.config.minToolCallsBeforeResult {
					remaining := a.config.maxTurns - turns
					rejection := a.config.resultGateRejectionPrompt
					if rejection == "" {
						rejection = fmt.Sprintf(
							"Submission rejected: you have only made %d tool calls so far (minimum %d required). You have %d turns remaining. Read more module audits, verify auth guards, and trace data flows before submitting.",
							callsSoFar, a.config.minToolCallsBeforeResult, remaining,
						)
					} else {
						rejection = fmt.Sprintf("%s (tool calls so far: %d/%d, turns remaining: %d)",
							rejection, callsSoFar, a.config.minToolCallsBeforeResult, remaining,
						)
					}
					a.config.logger.Info("result tool gate: rejecting early submission",
						"tool_calls", callsSoFar,
						"min_required", a.config.minToolCallsBeforeResult,
						"turns", turns,
						"remaining_turns", remaining,
					)
					toolUseBlocks = append(toolUseBlocks[:idx], toolUseBlocks[idx+1:]...)
					messages = append(messages, Message{
						Role: "user",
						ToolResults: []ToolResult{{
							ToolUseID: block.ID,
							Content:   []ToolResultContent{TextContent(rejection)},
							IsError:   true,
						}},
					})
					break
				}

				if out, ok := a.extractResultToolOutput(block); ok {
					a.config.logger.Info("result tool intercepted",
						"tool", block.Name,
						"turns", turns,
						"content_len", len(out.Content),
						"extra_fields", len(out.Metadata),
						"total_tool_calls", totalToolCalls.Load(),
					)
					return &Result{
						Response:         out.Content,
						Metadata:         out.Metadata,
						Messages:         messages,
						TotalUsage:       totalUsage,
						Turns:            turns,
						TotalRetryCount:  totalRetryCount,
						TotalRetryWaitMS: totalRetryWaitMS,
					}, nil
				}
			}
		}

		// Execute all tools in parallel (with optional concurrency limit)
		toolResults := make([]ToolResult, len(toolUseBlocks))
		g, gctx := errgroup.WithContext(ctx)
		if a.config.maxParallelTools > 0 {
			g.SetLimit(a.config.maxParallelTools)
		}
		for i, block := range toolUseBlocks {
			g.Go(func() error {
				select {
				case <-gctx.Done():
					return gctx.Err()
				default:
				}

				tool, ok := toolMap[block.Name]
				if !ok {
					a.config.logger.Warn("unknown tool requested", "name", block.Name)
					toolResults[i] = ToolResult{
						ToolUseID: block.ID,
						Content:   []ToolResultContent{TextContent(fmt.Sprintf("error: unknown tool %q", block.Name))},
						IsError:   true,
					}
					return nil
				}

				inputStr := string(block.Input)
				if len(inputStr) > 200 {
					inputStr = inputStr[:200] + "..."
				}
				a.config.logger.Info("executing tool", "name", block.Name, "input", inputStr)

				output, execErr := tool.Execute(gctx, block.Input)
				if execErr != nil {
					a.config.logger.Warn("tool execution failed", "name", block.Name, "error", execErr)
					errContent := []ToolResultContent{TextContent(fmt.Sprintf("error: %s", execErr.Error()))}
					toolResults[i] = ToolResult{
						ToolUseID: block.ID,
						Content:   errContent,
						IsError:   true,
					}
					if a.config.onToolCall != nil {
						a.config.onToolCall(ToolCallEvent{Turn: turns, Name: block.Name, Input: block.Input, Output: errContent, IsError: true})
					}
				} else {
					a.config.logger.Debug("tool executed", "name", block.Name, "output_len", len(output))
					totalToolCalls.Add(1)
					toolResults[i] = ToolResult{
						ToolUseID: block.ID,
						Content:   output,
						IsError:   false,
					}
					if a.config.onToolCall != nil {
						a.config.onToolCall(ToolCallEvent{Turn: turns, Name: block.Name, Input: block.Input, Output: output, IsError: false})
					}
				}
				return nil
			})
		}

		if err := g.Wait(); err != nil {
			return nil, fmt.Errorf("turn %d tool execution: %w", turns, err)
		}

		toolResultMsg := Message{
			Role:        "user",
			ToolResults: toolResults,
		}

		if a.config.resultToolName != "" && a.config.maxTurns > 3 {
			earlyNudgeTurn := a.config.maxTurns * 70 / 100
			if earlyNudgeTurn < 3 {
				earlyNudgeTurn = 3
			}
			remaining := a.config.maxTurns - turns
			if turns == earlyNudgeTurn {
				a.config.logger.Info("injecting early deadline nudge", "turns", turns, "max_turns", a.config.maxTurns, "remaining", remaining)
				toolResultMsg.Content = []ContentBlock{{
					Type: "text",
					Text: fmt.Sprintf("REMINDER: You have %d turns remaining (out of %d). Start wrapping up your analysis and call %s soon. A thorough partial audit is better than hitting the turn limit with nothing submitted.", remaining, a.config.maxTurns, a.config.resultToolName),
				}}
			} else if turns == a.config.maxTurns-1 {
				a.config.logger.Info("injecting final deadline nudge", "turns", turns, "max_turns", a.config.maxTurns)
				toolResultMsg.Content = []ContentBlock{{
					Type: "text",
					Text: fmt.Sprintf("CRITICAL: This is your LAST turn. You MUST call %s NOW with everything you have. Do NOT call any other tools.", a.config.resultToolName),
				}}
			}
		}

		messages = append(messages, toolResultMsg)
	}

	// Max turns exhausted — force one final submission-only call.
	// Strip all tools except the result tool so the model MUST submit.
	if a.config.resultToolName != "" {
		a.config.logger.Info("forcing submission-only turn (all exploration tools removed)",
			"turns", turns,
			"max_turns", a.config.maxTurns,
		)

		var resultToolOnly []Tool
		for _, t := range a.config.tools {
			if t.Name() == a.config.resultToolName {
				resultToolOnly = append(resultToolOnly, t)
				break
			}
		}

		if len(resultToolOnly) > 0 {
			// The last assistant message may have pending tool_use blocks that
			// need tool_result responses. Provide stub results so the API
			// doesn't reject the conversation for missing tool results.
			if len(messages) > 0 {
				lastMsg := messages[len(messages)-1]
				if lastMsg.Role == "assistant" {
					var stubs []ToolResult
					for _, block := range lastMsg.Content {
						if block.Type == "tool_use" {
							stubs = append(stubs, ToolResult{
								ToolUseID: block.ID,
								Content:   []ToolResultContent{{Type: "text", Text: "[turn limit reached — result discarded]"}},
							})
						}
					}
					if len(stubs) > 0 {
						messages = append(messages, Message{
							Role:        "user",
							ToolResults: stubs,
							Content: []ContentBlock{{
								Type: "text",
								Text: fmt.Sprintf("Turn limit reached. You MUST now call %s with your complete findings from all previous exploration. Synthesize everything you have learned into a comprehensive output.", a.config.resultToolName),
							}},
						})
					}
				}
			}
			if messages[len(messages)-1].Role != "user" {
				messages = append(messages, Message{
					Role: "user",
					Content: []ContentBlock{{
						Type: "text",
						Text: fmt.Sprintf("Turn limit reached. You MUST now call %s with your complete findings.", a.config.resultToolName),
					}},
				})
			}

			turns++
			resp, err := a.client.Send(ctx, &ClientRequest{
				System:          a.config.systemPrompt,
				Messages:        messages,
				Tools:           resultToolOnly,
				MaxTokens:       a.config.maxTokens,
				ForceToolChoice: a.config.resultToolName,
			})
			if err == nil {
				totalUsage.InputTokens += resp.Usage.InputTokens
				totalUsage.OutputTokens += resp.Usage.OutputTokens
				totalUsage.CacheCreationInputTokens += resp.Usage.CacheCreationInputTokens
				totalUsage.CacheReadInputTokens += resp.Usage.CacheReadInputTokens

				messages = append(messages, Message{Role: "assistant", Content: resp.Content})

			for _, block := range resp.Content {
				if block.Type == "tool_use" && block.Name == a.config.resultToolName {
					if out, ok := a.extractResultToolOutput(block); ok {
						a.config.logger.Info("forced submission succeeded",
							"turns", turns,
							"content_len", len(out.Content),
						)
						return &Result{
							Response:         out.Content,
							Metadata:         out.Metadata,
							Messages:         messages,
							TotalUsage:       totalUsage,
							Turns:            turns,
							TotalRetryCount:  totalRetryCount,
							TotalRetryWaitMS: totalRetryWaitMS,
						}, nil
					}
				}
			}
			}
		}
	}

	text := extractTextFromMessages(messages)
	a.config.logger.Warn("max turns reached (forced submission also failed)",
		"turns", turns,
		"total_input_tokens", totalUsage.InputTokens,
		"total_output_tokens", totalUsage.OutputTokens,
	)
	return &Result{
		Response:         text,
		Messages:         messages,
		TotalUsage:       totalUsage,
		Turns:            turns,
		TotalRetryCount:  totalRetryCount,
		TotalRetryWaitMS: totalRetryWaitMS,
	}, ErrMaxTurnsReached
}

type resultToolOutput struct {
	Content  string
	Metadata map[string]string
}

func (a *Agent) extractResultToolOutput(block ContentBlock) (resultToolOutput, bool) {
	var inputMap map[string]json.RawMessage
	if err := json.Unmarshal(block.Input, &inputMap); err != nil {
		return resultToolOutput{}, false
	}
	raw, ok := inputMap[a.config.resultFieldName]
	if !ok {
		return resultToolOutput{}, false
	}
	out := resultToolOutput{Content: extractResultField(raw)}
	if len(a.config.resultExtraFields) > 0 {
		out.Metadata = make(map[string]string, len(a.config.resultExtraFields))
		for _, field := range a.config.resultExtraFields {
			if rawField, ok := inputMap[field]; ok {
				out.Metadata[field] = extractResultField(rawField)
			}
		}
	}
	return out, true
}

func extractText(blocks []ContentBlock) string {
	var parts []string
	for _, b := range blocks {
		if b.Type == "text" && b.Text != "" {
			parts = append(parts, b.Text)
		}
	}
	return strings.Join(parts, "\n")
}

func extractTextFromMessages(messages []Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == "assistant" {
			return extractText(messages[i].Content)
		}
	}
	return ""
}

// extractResultField extracts a string value from raw JSON. If the JSON is a
// JSON string, it is unquoted. Otherwise the raw JSON is returned as-is,
// allowing result tools to return arrays, objects, or other structured data
// (e.g. batch documentation results) that the caller can parse further.
func extractResultField(raw json.RawMessage) string {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}
