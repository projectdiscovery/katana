package auth_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/projectdiscovery/katana/internal/testutils/authlab"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth"
	"github.com/stretchr/testify/require"
)

func skipIfNoBrowser(t *testing.T) {
	t.Helper()
	if path, found := launcher.LookPath(); !found || path == "" {
		t.Skip("chrome/chromium not found, skipping recorded-flow e2e")
	}
}

func launchPage(t *testing.T) (*rod.Browser, *rod.Page) {
	t.Helper()
	skipIfNoBrowser(t)

	path, _ := launcher.LookPath()
	u := launcher.New().
		Bin(path).
		Headless(true).
		Leakless(true).
		Set("no-sandbox", "true").
		MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()
	t.Cleanup(browser.MustClose)

	page, err := browser.Page(proto.TargetCreateTarget{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = page.Close() })
	return browser, page
}

func writeFlow(t *testing.T, contents string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "flow.json")
	require.NoError(t, os.WriteFile(path, []byte(contents), 0o600))
	return path
}

func assertSecretReachable(t *testing.T, page *rod.Page, lab *authlab.Lab) {
	t.Helper()
	require.NoError(t, page.Navigate(lab.URL+"/app/secret"))
	require.NoError(t, page.WaitLoad())
	html, err := page.HTML()
	require.NoError(t, err)
	require.Contains(t, html, authlab.SecretMarker, "session from recorded flow should unlock /app/secret")
	require.GreaterOrEqual(t, lab.SecretHits.Load(), int64(1))
}

func TestE2E_RecordedFlow_SimpleFormLogin(t *testing.T) {
	lab, err := authlab.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	_, page := launchPage(t)
	path := writeFlow(t, authlab.ChromeRecordingSimple(lab.URL))

	steps, err := auth.StepsFromFile(path, authlab.Username, authlab.Password)
	require.NoError(t, err)
	require.NotEmpty(t, steps)
	require.True(t, auth.NeedsCredentials(steps))

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	require.NoError(t, auth.RunLoginSteps(ctx, page, steps, authlab.Username, authlab.Password, 5*time.Second))
	require.GreaterOrEqual(t, lab.LoginPosts.Load(), int64(1))
	assertSecretReachable(t, page, lab)
}

func TestE2E_RecordedFlow_UsernameFirst(t *testing.T) {
	lab, err := authlab.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	_, page := launchPage(t)
	path := writeFlow(t, authlab.ChromeRecordingStep(lab.URL))

	steps, err := auth.StepsFromFile(path, authlab.Username, authlab.Password)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	require.NoError(t, auth.RunLoginSteps(ctx, page, steps, authlab.Username, authlab.Password, 5*time.Second))
	assertSecretReachable(t, page, lab)
}

func TestE2E_RecordedFlow_SPA(t *testing.T) {
	lab, err := authlab.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	_, page := launchPage(t)
	path := writeFlow(t, authlab.ChromeRecordingSPA(lab.URL))

	steps, err := auth.StepsFromFile(path, authlab.Username, authlab.Password)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	require.NoError(t, auth.RunLoginSteps(ctx, page, steps, authlab.Username, authlab.Password, 5*time.Second))
	assertSecretReachable(t, page, lab)
}

func TestE2E_RecordedFlow_ExplicitSteps(t *testing.T) {
	lab, err := authlab.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	_, page := launchPage(t)
	path := writeFlow(t, authlab.ExplicitStepsSimple(lab.URL))

	steps, err := auth.StepsFromFile(path, authlab.Username, authlab.Password)
	require.NoError(t, err)
	require.Equal(t, "navigate", steps[0].Action)
	require.Equal(t, "{{username}}", steps[1].Value)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	require.NoError(t, auth.RunLoginSteps(ctx, page, steps, authlab.Username, authlab.Password, 5*time.Second))
	assertSecretReachable(t, page, lab)
}

func TestE2E_RecordedFlow_WrongPasswordDoesNotUnlockSecret(t *testing.T) {
	lab, err := authlab.Start()
	require.NoError(t, err)
	t.Cleanup(func() { _ = lab.Close() })

	_, page := launchPage(t)
	path := writeFlow(t, authlab.ChromeRecordingSimple(lab.URL))

	steps, err := auth.StepsFromFile(path, authlab.Username, authlab.Password)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()
	// Replay with wrong password — form submit may "succeed" as clicks, but
	// the session cookie must not be issued.
	_ = auth.RunLoginSteps(ctx, page, steps, authlab.Username, "wrong-password", 5*time.Second)

	require.NoError(t, page.Navigate(lab.URL+"/app/secret"))
	require.NoError(t, page.WaitLoad())
	html, err := page.HTML()
	require.NoError(t, err)
	require.NotContains(t, html, authlab.SecretMarker)
	require.Equal(t, int64(0), lab.SecretHits.Load())
}
