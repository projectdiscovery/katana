package captcha

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInjectionScript(t *testing.T) {
	tests := []struct {
		name     string
		provider Provider
		wantErr  bool
		contains string
	}{
		{
			name:     "recaptcha v2",
			provider: ProviderRecaptchaV2,
			contains: "g-recaptcha-response",
		},
		{
			name:     "recaptcha v3",
			provider: ProviderRecaptchaV3,
			contains: "g-recaptcha-response",
		},
		{
			name:     "recaptcha v2 enterprise",
			provider: ProviderRecaptchaV2Enterprise,
			contains: "g-recaptcha-response",
		},
		{
			name:     "recaptcha v3 enterprise",
			provider: ProviderRecaptchaV3Enterprise,
			contains: "g-recaptcha-response",
		},
		{
			name:     "turnstile",
			provider: ProviderTurnstile,
			contains: "cf-turnstile-response",
		},
		{
			name:     "hcaptcha",
			provider: ProviderHCaptcha,
			contains: "h-captcha-response",
		},
		{
			name:     "unknown provider",
			provider: "unknown",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			js, err := injectionScript(tt.provider)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, js, tt.contains)
		})
	}
}
