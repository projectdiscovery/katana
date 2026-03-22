package capsolver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/headless/captcha"
)

const (
	defaultPollInterval = 3 * time.Second
	defaultTimeout      = 120 * time.Second
)

var baseURL = "https://api.capsolver.com"

func SetBaseURL(url string) {
	baseURL = url
}

var taskTypes = map[captcha.Provider]string{
	captcha.ProviderRecaptchaV2:           "ReCaptchaV2TaskProxyLess",
	captcha.ProviderRecaptchaV3:           "ReCaptchaV3TaskProxyLess",
	captcha.ProviderRecaptchaV2Enterprise: "ReCaptchaV2EnterpriseTaskProxyLess",
	captcha.ProviderRecaptchaV3Enterprise: "ReCaptchaV3EnterpriseTaskProxyLess",
	captcha.ProviderTurnstile:             "AntiTurnstileTaskProxyLess",
	captcha.ProviderHCaptcha:              "HCaptchaTaskProxyLess",
}

type Solver struct {
	apiKey string
	client *http.Client
}

func init() {
	captcha.RegisterSolver("capsolver", func(apiKey string) (captcha.Solver, error) {
		return New(apiKey), nil
	})
}

func New(apiKey string) *Solver {
	return &Solver{
		apiKey: apiKey,
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *Solver) Solve(ctx context.Context, info *captcha.Info) (*captcha.Solution, error) {
	taskType, ok := taskTypes[info.Provider]
	if !ok {
		return nil, fmt.Errorf("unsupported captcha provider for capsolver: %s", info.Provider)
	}

	task := map[string]any{
		"type":       taskType,
		"websiteURL": info.PageURL,
		"websiteKey": info.SiteKey,
	}
	if (info.Provider == captcha.ProviderRecaptchaV3 || info.Provider == captcha.ProviderRecaptchaV3Enterprise) && info.Action != "" {
		task["pageAction"] = info.Action
	}

	taskID, err := s.createTask(ctx, task)
	if err != nil {
		return nil, fmt.Errorf("create task: %w", err)
	}

	return s.pollResult(ctx, taskID, info.Provider)
}

type createTaskRequest struct {
	ClientKey string         `json:"clientKey"`
	Task      map[string]any `json:"task"`
}

type createTaskResponse struct {
	ErrorID          int    `json:"errorId"`
	ErrorCode        string `json:"errorCode"`
	ErrorDescription string `json:"errorDescription"`
	TaskID           string `json:"taskId"`
}

type getTaskResultRequest struct {
	ClientKey string `json:"clientKey"`
	TaskID    string `json:"taskId"`
}

type getTaskResultResponse struct {
	ErrorID          int            `json:"errorId"`
	ErrorCode        string         `json:"errorCode"`
	ErrorDescription string         `json:"errorDescription"`
	Status           string         `json:"status"`
	Solution         map[string]any `json:"solution"`
}

func (s *Solver) createTask(ctx context.Context, task map[string]any) (string, error) {
	body, err := json.Marshal(createTaskRequest{
		ClientKey: s.apiKey,
		Task:      task,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/createTask", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var result createTaskResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", err
	}
	if result.ErrorID != 0 {
		return "", fmt.Errorf("capsolver error %s: %s", result.ErrorCode, result.ErrorDescription)
	}
	return result.TaskID, nil
}

func (s *Solver) pollResult(ctx context.Context, taskID string, provider captcha.Provider) (*captcha.Solution, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	ticker := time.NewTicker(defaultPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("captcha solve timed out after %s", defaultTimeout)
		case <-ticker.C:
			result, err := s.getTaskResult(ctx, taskID)
			if err != nil {
				return nil, err
			}
			if result.ErrorID != 0 {
				return nil, fmt.Errorf("capsolver error %s: %s", result.ErrorCode, result.ErrorDescription)
			}
			if result.Status != "ready" {
				continue
			}
			return extractToken(result.Solution, provider)
		}
	}
}

func (s *Solver) getTaskResult(ctx context.Context, taskID string) (*getTaskResultResponse, error) {
	body, err := json.Marshal(getTaskResultRequest{
		ClientKey: s.apiKey,
		TaskID:    taskID,
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, baseURL+"/getTaskResult", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result getTaskResultResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func extractToken(solution map[string]any, provider captcha.Provider) (*captcha.Solution, error) {
	// capsolver returns turnstile tokens under "token", everything else under "gRecaptchaResponse"
	key := "gRecaptchaResponse"
	if provider == captcha.ProviderTurnstile {
		key = "token"
	}

	token, ok := solution[key].(string)
	if !ok || token == "" {
		return nil, fmt.Errorf("no token found in capsolver solution (key=%s)", key)
	}

	return &captcha.Solution{Token: token, Provider: provider}, nil
}
