package captcha

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegration_DetectRealCaptchaPages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	browser := setupBrowser(t)

	tests := []struct {
		name         string
		url          string
		wantProvider Provider
	}{
		{
			name:         "google recaptcha v2 demo",
			url:          "https://www.google.com/recaptcha/api2/demo",
			wantProvider: ProviderRecaptchaV2,
		},
		{
			name:         "hcaptcha official demo",
			url:          "https://accounts.hcaptcha.com/demo",
			wantProvider: ProviderHCaptcha,
		},
		{
			name:         "2captcha turnstile demo",
			url:          "https://2captcha.com/demo/cloudflare-turnstile",
			wantProvider: ProviderTurnstile,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page := browser.MustPage(tt.url)
			defer page.MustClose()
			page.MustWaitLoad()

			info, err := Identify(page)
			require.NoError(t, err)
			require.NotNil(t, info, "expected captcha to be detected on %s", tt.url)

			t.Logf("detected: provider=%s sitekey=%s url=%s", info.Provider, info.SiteKey, info.PageURL)

			assert.Equal(t, tt.wantProvider, info.Provider)
			assert.NotEmpty(t, info.SiteKey)
		})
	}
}
