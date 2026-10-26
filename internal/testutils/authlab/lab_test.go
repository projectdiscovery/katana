package authlab

import (
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLab_GatesSecretAndAcceptsFormLogin(t *testing.T) {
	lab, err := Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	resp, err := http.Get(lab.URL + "/app/secret")
	require.NoError(t, err)
	require.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	_ = resp.Body.Close()

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}

	form := url.Values{"email": {Username}, "password": {Password}}
	resp, err = client.PostForm(lab.URL+"/login", form)
	require.NoError(t, err)
	require.Equal(t, http.StatusFound, resp.StatusCode)
	_ = resp.Body.Close()

	client.CheckRedirect = nil
	resp, err = client.Get(lab.URL + "/app/secret")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	require.Contains(t, string(body), SecretMarker)
	require.GreaterOrEqual(t, lab.LoginPosts.Load(), int64(1))
	require.GreaterOrEqual(t, lab.SecretHits.Load(), int64(1))
}

func TestLab_APILogin(t *testing.T) {
	lab, err := Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	jar, err := cookiejar.New(nil)
	require.NoError(t, err)
	client := &http.Client{Jar: jar}

	resp, err := client.Post(lab.URL+"/api/login", "application/json",
		strings.NewReader(`{"email":"`+Username+`","password":"`+Password+`"}`))
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	_ = resp.Body.Close()

	resp, err = client.Get(lab.URL + "/app/dashboard")
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestRecordingHelpersProduceValidJSONShape(t *testing.T) {
	for name, raw := range map[string]string{
		"simple":   ChromeRecordingSimple("http://127.0.0.1:9"),
		"step":     ChromeRecordingStep("http://127.0.0.1:9"),
		"spa":      ChromeRecordingSPA("http://127.0.0.1:9"),
		"explicit": ExplicitStepsSimple("http://127.0.0.1:9"),
	} {
		t.Run(name, func(t *testing.T) {
			require.Contains(t, raw, "steps")
			require.Contains(t, raw, "/login")
			if name == "explicit" {
				require.Contains(t, raw, "{{username}}")
				require.Contains(t, raw, "{{password}}")
			} else {
				require.Contains(t, raw, Username)
			}
		})
	}
}
