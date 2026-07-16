package captcha

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestE2E_RecaptchaV2_Injection(t *testing.T) {
	browser := setupBrowser(t)

	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = r.ParseForm()
			received <- r.FormValue("g-recaptcha-response")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><div id="done">OK</div></body></html>`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
			<form method="POST">
				<div class="g-recaptcha" data-sitekey="6LcTestKey" data-callback="onSolved"></div>
				<textarea id="g-recaptcha-response" name="g-recaptcha-response" style="display:none"></textarea>
			</form>
			<div id="result"></div>
			<script>function onSolved(token) { document.getElementById('result').textContent = 'SOLVED:' + token; }</script>
		</body></html>`))
	}))
	t.Cleanup(server.Close)

	p := browser.MustPage(server.URL)
	defer p.MustClose()
	p.MustWaitLoad()

	info, err := Identify(p)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, ProviderRecaptchaV2, info.Provider)
	assert.Equal(t, "6LcTestKey", info.SiteKey)

	err = injectToken(p, &Solution{Token: "test-token-abc", Provider: ProviderRecaptchaV2})
	require.NoError(t, err)

	p.MustWaitLoad()
	token := <-received
	assert.Equal(t, "test-token-abc", token)
}

func TestE2E_Turnstile_Injection(t *testing.T) {
	browser := setupBrowser(t)

	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = r.ParseForm()
			received <- r.FormValue("cf-turnstile-response")
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><div id="done">OK</div></body></html>`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
			<form method="POST">
				<div class="cf-turnstile" data-sitekey="0x4AAATurnstileKey" data-callback="onTurnstile"></div>
				<input type="hidden" name="cf-turnstile-response" value="">
			</form>
			<div id="result"></div>
			<script>function onTurnstile(token) { document.getElementById('result').textContent = 'TURNSTILE:' + token; }</script>
		</body></html>`))
	}))
	t.Cleanup(server.Close)

	p := browser.MustPage(server.URL)
	defer p.MustClose()
	p.MustWaitLoad()

	info, err := Identify(p)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, ProviderTurnstile, info.Provider)
	assert.Equal(t, "0x4AAATurnstileKey", info.SiteKey)

	err = injectToken(p, &Solution{Token: "test-token-def", Provider: ProviderTurnstile})
	require.NoError(t, err)

	p.MustWaitLoad()
	token := <-received
	assert.Equal(t, "test-token-def", token)
}

func TestE2E_HCaptcha_Injection(t *testing.T) {
	browser := setupBrowser(t)

	received := make(chan map[string]string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = r.ParseForm()
			received <- map[string]string{
				"h-captcha-response":   r.FormValue("h-captcha-response"),
				"g-recaptcha-response": r.FormValue("g-recaptcha-response"),
			}
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><div id="done">OK</div></body></html>`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
			<form method="POST">
				<div class="h-captcha" data-sitekey="hcap-test-key" data-callback="onHCaptcha"></div>
				<textarea name="h-captcha-response" style="display:none"></textarea>
				<textarea name="g-recaptcha-response" style="display:none"></textarea>
			</form>
			<div id="result"></div>
			<script>function onHCaptcha(token) { document.getElementById('result').textContent = 'HCAPTCHA:' + token; }</script>
		</body></html>`))
	}))
	t.Cleanup(server.Close)

	p := browser.MustPage(server.URL)
	defer p.MustClose()
	p.MustWaitLoad()

	info, err := Identify(p)
	require.NoError(t, err)
	require.NotNil(t, info)
	assert.Equal(t, ProviderHCaptcha, info.Provider)
	assert.Equal(t, "hcap-test-key", info.SiteKey)

	err = injectToken(p, &Solution{Token: "test-token-ghi", Provider: ProviderHCaptcha})
	require.NoError(t, err)

	p.MustWaitLoad()
	vals := <-received
	assert.Equal(t, "test-token-ghi", vals["h-captcha-response"])
	assert.Equal(t, "test-token-ghi", vals["g-recaptcha-response"])
}

func TestE2E_RecaptchaV2_FormSubmitFallback(t *testing.T) {
	browser := setupBrowser(t)

	received := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			_ = r.ParseForm()
			received <- r.FormValue("g-recaptcha-response")
			_, _ = w.Write([]byte("OK"))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body>
			<form action="` + r.Host + `" method="POST">
				<div class="g-recaptcha" data-sitekey="6LcNoCallback"></div>
				<textarea id="g-recaptcha-response" name="g-recaptcha-response" style="display:none"></textarea>
			</form>
		</body></html>`))
	}))
	t.Cleanup(server.Close)

	p := browser.MustPage(server.URL)
	defer p.MustClose()
	p.MustWaitLoad()

	err := injectToken(p, &Solution{Token: "fallback-token", Provider: ProviderRecaptchaV2})
	require.NoError(t, err)

	p.MustWaitLoad()
	token := <-received
	assert.Equal(t, "fallback-token", token)
}
