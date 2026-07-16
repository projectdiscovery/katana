package captcha

import (
	"github.com/go-rod/rod"
	captchajs "github.com/projectdiscovery/katana/pkg/engine/headless/captcha/js"
)

type Provider string

const (
	ProviderRecaptchaV2           Provider = "recaptchav2"
	ProviderRecaptchaV3           Provider = "recaptchav3"
	ProviderRecaptchaV2Enterprise Provider = "recaptchav2enterprise"
	ProviderRecaptchaV3Enterprise Provider = "recaptchav3enterprise"
	ProviderTurnstile             Provider = "turnstile"
	ProviderHCaptcha              Provider = "hcaptcha"
)

type Info struct {
	Provider Provider
	SiteKey  string
	PageURL  string
	Action   string
}

func Identify(page *rod.Page) (*Info, error) {
	pageURL, err := page.Eval("() => window.location.href")
	if err != nil {
		return nil, err
	}

	result, err := page.Eval(captchajs.IdentifyJS)
	if err != nil {
		return nil, err
	}

	if result.Value.Nil() {
		return nil, nil
	}

	return &Info{
		Provider: Provider(result.Value.Get("provider").Str()),
		SiteKey:  result.Value.Get("sitekey").Str(),
		PageURL:  pageURL.Value.Str(),
		Action:   result.Value.Get("action").Str(),
	}, nil
}
