// Package agentlogin provides an AI-powered login agent that uses an LLM
// to figure out how to log into web applications when heuristic detection fails.
package agentlogin

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/agent"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth/recipe"
)

const loginSystemPrompt = `You are a login automation agent. You MUST log into the web application using the tools provided. NEVER respond with text — ALWAYS call a tool.

CRITICAL: Many login forms are multi-step (Clerk, Auth0, Okta, etc.):
- Step 1: You see ONLY an email/username field. Type the email and click the submit/continue button.
- Step 2: AFTER clicking, the form changes to show a password field. Call get_page to see the updated form.
- Step 3: Type the password and click the sign-in button.
You MUST call get_page after EVERY click to see what changed on the page.

WORKFLOW — follow this EXACTLY:
1. Call get_page to see the current form
2. Type the username/email into the email field using type_text
3. Click the submit/continue button using click
4. Call get_page to see the updated form (password field should now appear)
5. If you see a password field, type the password using type_text
6. Click the sign-in/submit button using click
7. Call get_page to verify login succeeded (URL should change, dashboard content)
8. Call save_result with the steps you took

CREDENTIALS:
- For the email/username field, call: type_text(selector, "{{username}}")
- For the password field, call: type_text(selector, "{{password}}")
- These placeholders are automatically replaced with actual values. NEVER type actual credentials.

SELECTOR TIPS:
- Use #id selectors first: "#identifier-field", "#password-field", "#email"
- For buttons: "button[type='submit']", "button.cl-formButtonPrimary", or the first <button> in the form
- If a selector fails, try a different one from the get_page output`

func init() {
	auth.RegisterLoginAgent("anthropic", func(apiKey string) (auth.LoginAgent, error) {
		return NewAnthropicLoginAgent(apiKey, agent.ModelSonnet4_6), nil
	})
}

// AnthropicLoginAgent implements auth.LoginAgent using the agent framework.
type AnthropicLoginAgent struct {
	apiKey string
	model  string
	logger *slog.Logger
}

func NewAnthropicLoginAgent(apiKey, model string) *AnthropicLoginAgent {
	return &AnthropicLoginAgent{
		apiKey: apiKey,
		model:  model,
		logger: slog.Default(),
	}
}

func (a *AnthropicLoginAgent) Login(ctx context.Context, toolkit auth.BrowserToolkit, config *auth.AuthConfig) (*auth.LoginResult, error) {
	gologger.Info().Msgf("[auth-agent] Login() entered — model=%s key_prefix=%s", a.model, truncate(a.apiKey, 10))
	client := agent.NewAnthropicAgentClient(a.apiKey, agent.WithModel(a.model))

	tools := buildBrowserTools(toolkit, config.Credentials.GetUsername(), config.Credentials.Password)

	resultTool := agent.NewMultiFieldResultTool(
		"save_result",
		"Save the login result. Call this AFTER you have attempted to log in and verified the outcome.",
		map[string]string{
			"success": "true if login succeeded (page changed to dashboard/home), false if it failed",
			"steps":   `JSON array of recipe steps taken, e.g.: [{"action":"type","selector":"#email","value":"{{username}}","field":"username"},{"action":"click","selector":"button.submit","field":"submit"}]`,
			"error":   "Error description if login failed, empty string if succeeded",
		},
	)
	tools = append(tools, resultTool)

	loginAgent := agent.New(client,
		agent.WithSystemPrompt(loginSystemPrompt),
		agent.WithTools(tools...),
		agent.WithMaxTurns(12),
		agent.WithTokenBudget(30000),
		agent.WithMaxTokensPerCall(4096),
		agent.WithResultTool("save_result", "steps", "success", "error"),
		agent.WithMinToolCallsBeforeResult(2,
			"You must call get_page and interact with the form before saving. Type credentials and click submit first."),
		agent.WithOnTurn(func(ev agent.TurnEvent) {
			gologger.Info().Msgf("[auth-agent] turn=%d stop=%s tokens=%d/%d",
				ev.Turn, ev.Response.StopReason, ev.TotalUsage.InputTokens, ev.TotalUsage.OutputTokens)
			// Print assistant text responses
			for _, block := range ev.Response.Content {
				if block.Type == "text" && block.Text != "" {
					gologger.Info().Msgf("[auth-agent] <<< assistant: %s", truncate(block.Text, 500))
				}
				if block.Type == "tool_use" {
					inputStr := string(block.Input)
					if len(inputStr) > 300 {
						inputStr = inputStr[:300] + "..."
					}
					gologger.Info().Msgf("[auth-agent] <<< tool_use: %s(%s)", block.Name, inputStr)
				}
			}
		}),
		agent.WithOnToolCall(func(ev agent.ToolCallEvent) {
			for _, c := range ev.Output {
				if c.Type == "text" {
					prefix := ">>>"
					if ev.IsError {
						prefix = ">>> ERROR"
					}
					gologger.Info().Msgf("[auth-agent] %s %s: %s", prefix, ev.Name, truncate(c.Text, 2000))
				}
			}
		}),
		agent.WithAgentLogger(a.logger),
	)

	// Get initial page state to include in the message
	currentURL, _ := toolkit.GetCurrentURL()

	userMsg := fmt.Sprintf(
		"Log into the web application. The browser is currently at: %s\n\nCredentials are available as variables:\n- Use {{username}} when typing into the email/username field\n- Use {{password}} when typing into the password field\n\nThe type_text tool will automatically substitute these with actual values. Start by calling get_page to see the login form.",
		currentURL,
	)

	gologger.Info().Msgf("[auth-agent] Starting AI login — url=%s model=%s tools=%d", currentURL, a.model, len(tools))

	result, err := loginAgent.Run(ctx, userMsg)
	if err != nil {
		errMsg := fmt.Sprintf("agent run error: %s", err)
		gologger.Warning().Msgf("[auth-agent] Agent run FAILED: %s", errMsg)
		return &auth.LoginResult{
			Success: false,
			Error:   errMsg,
		}, fmt.Errorf("ai login agent: %w", err)
	}

	gologger.Info().Msgf("[auth-agent] Agent completed — turns=%d input_tokens=%d output_tokens=%d",
		result.Turns, result.TotalUsage.InputTokens, result.TotalUsage.OutputTokens)

	// Parse result
	loginResult := &auth.LoginResult{}

	if successStr, ok := result.Metadata["success"]; ok {
		loginResult.Success = successStr == "true"
	}
	if errStr, ok := result.Metadata["error"]; ok {
		loginResult.Error = errStr
	}

	// If agent returned text response instead of using result tool, treat as failure
	if result.Response == "" && len(result.Metadata) == 0 {
		loginResult.Success = false
		loginResult.Error = "agent did not produce structured result"
		// Extract what the agent actually said
		for i := len(result.Messages) - 1; i >= 0; i-- {
			msg := result.Messages[i]
			if msg.Role == "assistant" {
				for _, block := range msg.Content {
					if block.Type == "text" && block.Text != "" {
						loginResult.Error = fmt.Sprintf("agent said: %s", truncate(block.Text, 300))
						gologger.Warning().Msgf("[auth-agent] Agent responded with text instead of tools: %s", truncate(block.Text, 300))
						break
					}
				}
				break
			}
		}
	}

	// Parse recipe steps
	stepsJSON := result.Response
	if stepsJSON != "" {
		var steps []recipe.Step
		if err := json.Unmarshal([]byte(stepsJSON), &steps); err == nil && len(steps) > 0 {
			loginResult.Recipe = &recipe.Recipe{
				LoginURL:  currentURL,
				Steps:     steps,
				CreatedAt: time.Now(),
				Version:   1,
			}
			gologger.Info().Msgf("[auth-agent] Recipe extracted — %d steps", len(steps))
		} else if err != nil {
			gologger.Debug().Msgf("[auth-agent] Failed to parse recipe steps: %s (raw: %s)", err, truncate(stepsJSON, 200))
		}
	}

	// Extract session if login succeeded
	if loginResult.Success {
		cookies, err := toolkit.GetCookies()
		if err == nil {
			loginResult.Session = &auth.SessionState{Cookies: cookies}
			gologger.Info().Msgf("[auth-agent] Session extracted — %d cookies", len(cookies))
		}
	}

	return loginResult, nil
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// substituteVars replaces {{username}} and {{password}} with real values.
func substituteVars(text, username, password string) string {
	text = strings.ReplaceAll(text, "{{username}}", username)
	text = strings.ReplaceAll(text, "{{password}}", password)
	return text
}

func buildBrowserTools(toolkit auth.BrowserToolkit, username, password string) []agent.Tool {
	return []agent.Tool{
		&agent.ToolFunc{
			ToolName:        "get_page",
			ToolDescription: "Get the current page: URL, login-relevant form fields (inputs, buttons), and visible text. Call after every action.",
			ToolSchema:      agent.InputSchema{Properties: map[string]any{}},
			Fn: func(ctx context.Context, input json.RawMessage) (string, error) {
				url, _ := toolkit.GetCurrentURL()
				forms, _ := toolkit.GetForms()
				text, _ := toolkit.GetVisibleText()
				if len(text) > 300 {
					text = text[:300]
				}

				// Only include buttons from interactive elements (not links/scripts)
				elements, _ := toolkit.GetInteractiveElements()
				var buttons []auth.ElementView
				for _, el := range elements {
					tag := el.TagName
					if tag == "BUTTON" || tag == "INPUT" || el.Role == "button" {
						buttons = append(buttons, el)
					}
				}

				pageData := map[string]any{
					"url":          url,
					"forms":        forms,
					"buttons":      buttons,
					"visible_text": text,
				}
				data, _ := json.MarshalIndent(pageData, "", "  ")
				return string(data), nil
			},
		},

		&agent.ToolFunc{
			ToolName:        "type_text",
			ToolDescription: "Clear a field and type text into it. Use {{username}} or {{password}} as the text value — they will be substituted with actual credentials automatically. Use CSS selectors like '#email', '#identifier-field', 'input[type=\"password\"]'.",
			ToolSchema: agent.InputSchema{
				Properties: map[string]any{
					"selector": map[string]any{"type": "string", "description": "CSS selector for the input field"},
					"text":     map[string]any{"type": "string", "description": "Text to type. Use {{username}} or {{password}} for credentials."},
				},
				Required: []string{"selector", "text"},
			},
			Fn: func(ctx context.Context, input json.RawMessage) (string, error) {
				var p struct {
					Selector string `json:"selector"`
					Text     string `json:"text"`
				}
				if err := json.Unmarshal(input, &p); err != nil {
					return "", err
				}
				// Substitute credential placeholders with real values
				actualText := substituteVars(p.Text, username, password)
				if err := toolkit.TypeText(p.Selector, actualText); err != nil {
					return "", fmt.Errorf("type_text(%s): %w", p.Selector, err)
				}
				// Log with placeholder, not actual value
				return fmt.Sprintf("OK: typed %s into %s", p.Text, p.Selector), nil
			},
		},

		&agent.ToolFunc{
			ToolName:        "click",
			ToolDescription: "Click an element. After clicking, the page may navigate or update. Use CSS selectors like 'button[type=\"submit\"]', '#login-btn', 'button:has-text(\"Continue\")'.",
			ToolSchema: agent.InputSchema{
				Properties: map[string]any{
					"selector": map[string]any{"type": "string", "description": "CSS selector for the element to click"},
				},
				Required: []string{"selector"},
			},
			Fn: func(ctx context.Context, input json.RawMessage) (string, error) {
				var p struct {
					Selector string `json:"selector"`
				}
				if err := json.Unmarshal(input, &p); err != nil {
					return "", err
				}
				if err := toolkit.Click(p.Selector); err != nil {
					return "", fmt.Errorf("click(%s): %w", p.Selector, err)
				}
				toolkit.WaitForNavigation(3 * time.Second)
				url, _ := toolkit.GetCurrentURL()
				return fmt.Sprintf("OK: clicked %s — now at: %s", p.Selector, url), nil
			},
		},

		&agent.ToolFunc{
			ToolName:        "press_enter",
			ToolDescription: "Press Enter key to submit a form.",
			ToolSchema:      agent.InputSchema{Properties: map[string]any{}},
			Fn: func(ctx context.Context, input json.RawMessage) (string, error) {
				if err := toolkit.PressEnter(); err != nil {
					return "", err
				}
				toolkit.WaitForNavigation(3 * time.Second)
				url, _ := toolkit.GetCurrentURL()
				return "OK: pressed Enter — now at: " + url, nil
			},
		},

		&agent.ToolFunc{
			ToolName:        "get_cookies",
			ToolDescription: "List all browser cookies. Use after login to verify session cookies were set.",
			ToolSchema:      agent.InputSchema{Properties: map[string]any{}},
			Fn: func(ctx context.Context, input json.RawMessage) (string, error) {
				cookies, err := toolkit.GetCookies()
				if err != nil {
					return "", err
				}
				type cs struct {
					Name     string `json:"name"`
					Domain   string `json:"domain"`
					HTTPOnly bool   `json:"http_only"`
				}
				var summaries []cs
				for _, c := range cookies {
					summaries = append(summaries, cs{c.Name, c.Domain, c.HTTPOnly})
				}
				data, _ := json.MarshalIndent(summaries, "", "  ")
				return string(data), nil
			},
		},
	}
}
