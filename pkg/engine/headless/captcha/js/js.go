package js

import _ "embed"

var (
	//go:embed identify.js
	IdentifyJS string

	//go:embed inject-recaptcha.js
	InjectRecaptchaJS string

	//go:embed inject-turnstile.js
	InjectTurnstileJS string

	//go:embed inject-hcaptcha.js
	InjectHCaptchaJS string
)
