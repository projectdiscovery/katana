package captcha

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-rod/rod"
	ditcaptcha "github.com/happyhackingspace/dit/captcha"
	"github.com/projectdiscovery/gologger"
	captchajs "github.com/projectdiscovery/katana/pkg/engine/headless/captcha/js"
)

type Handler struct {
	solver Solver
}

func NewHandler(solverProvider, apiKey string) (*Handler, error) {
	solver, err := NewSolver(solverProvider, apiKey)
	if err != nil {
		return nil, fmt.Errorf("captcha solver init: %w", err)
	}

	return &Handler{
		solver: solver,
	}, nil
}

func (h *Handler) HandleIfCaptcha(ctx context.Context, page *rod.Page, pageHTML string) (bool, error) {
	if ditcaptcha.DetectCaptchaInHTML(pageHTML) == ditcaptcha.CaptchaTypeNone && !strings.Contains(pageHTML, "data-sitekey") {
		return false, nil
	}

	info, err := Identify(page)
	if err != nil {
		gologger.Debug().Msgf("captcha identification failed: %s", err)
	}

	if info == nil {
		return false, nil
	}

	return h.solveCaptcha(ctx, page, info)
}

func (h *Handler) solveCaptcha(ctx context.Context, page *rod.Page, info *Info) (bool, error) {
	gologger.Debug().Msgf("captcha detected: provider=%s sitekey=%s url=%s", info.Provider, info.SiteKey, info.PageURL)

	solution, err := h.solver.Solve(ctx, info)
	if err != nil {
		return true, fmt.Errorf("captcha solve: %w", err)
	}

	gologger.Debug().Msgf("captcha solved, injecting token: provider=%s", solution.Provider)

	if err := injectToken(page, solution); err != nil {
		return true, fmt.Errorf("captcha inject: %w", err)
	}

	return true, nil
}

func injectToken(page *rod.Page, solution *Solution) error {
	js, err := injectionScript(solution.Provider)
	if err != nil {
		return err
	}
	_, err = page.Eval(js, solution.Token)
	return err
}

func injectionScript(provider Provider) (string, error) {
	switch provider {
	case ProviderRecaptchaV2, ProviderRecaptchaV3, ProviderRecaptchaV2Enterprise, ProviderRecaptchaV3Enterprise:
		return captchajs.InjectRecaptchaJS, nil
	case ProviderTurnstile:
		return captchajs.InjectTurnstileJS, nil
	case ProviderHCaptcha:
		return captchajs.InjectHCaptchaJS, nil
	default:
		return "", fmt.Errorf("unsupported captcha provider for injection: %s", provider)
	}
}
