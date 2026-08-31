// Package authlab provides a self-hosted mini web app used to exercise
// Katana recorded-flow / auto-login authentication end-to-end.
//
// It exposes three login styles against the same cookie-gated app surface:
//
//  1. Classic HTML form POST  (/login)
//  2. Username-first multi-step (/login/step) — email → next → password
//  3. JS SPA login              (/login/spa)  — client-side panels + fetch
//
// Authenticated pages under /app/* require the session cookie set by a
// successful login. Tests assert that a recorded flow can unlock those pages
// for a subsequent crawl.
package authlab

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync/atomic"
)

const (
	// Username / Password are the only credentials the lab accepts.
	Username = "tester@katana.test"
	Password = "katana-rocks"

	CookieName  = "katana_session"
	CookieValue = "authenticated"

	// SecretMarker appears only on the cookie-gated /app/secret page.
	SecretMarker = "KATANA_AUTH_SECRET_OK"
)

// Lab is a running local auth testbed.
type Lab struct {
	URL      string
	listener net.Listener
	server   *http.Server

	LoginPosts    atomic.Int64
	SecretHits    atomic.Int64
	DashboardHits atomic.Int64
}

// Start launches the lab on 127.0.0.1 with an ephemeral port.
func Start() (*Lab, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("authlab: listen: %w", err)
	}
	lab := &Lab{
		URL:      fmt.Sprintf("http://%s", listener.Addr().String()),
		listener: listener,
	}
	mux := http.NewServeMux()
	lab.register(mux)
	lab.server = &http.Server{Handler: mux}
	go func() { _ = lab.server.Serve(listener) }()
	return lab, nil
}

// Close shuts the lab down.
func (l *Lab) Close() error {
	if l.server != nil {
		// Server.Close also closes the underlying listener.
		return l.server.Close()
	}
	if l.listener != nil {
		return l.listener.Close()
	}
	return nil
}

func (l *Lab) register(mux *http.ServeMux) {
	mux.HandleFunc("/", l.handleHome)
	mux.HandleFunc("/about-public", l.handleAboutPublic)
	mux.HandleFunc("/login", l.handleLogin)
	mux.HandleFunc("/login/step", l.handleLoginStep)
	mux.HandleFunc("/login/spa", l.handleLoginSPA)
	mux.HandleFunc("/api/login", l.handleAPILogin)
	mux.HandleFunc("/app/dashboard", l.handleDashboard)
	mux.HandleFunc("/app/secret", l.handleSecret)
	mux.HandleFunc("/app/settings", l.handleSettings)
	mux.HandleFunc("/logout", l.handleLogout)
}

func (l *Lab) authenticated(r *http.Request) bool {
	c, err := r.Cookie(CookieName)
	return err == nil && c != nil && c.Value == CookieValue
}

func (l *Lab) setSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    CookieValue,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
}

func (l *Lab) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     CookieName,
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
	})
}

func (l *Lab) handleHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	authLinks := `<p>You are anonymous. <a href="/login">Login</a> · <a href="/login/step">Step login</a> · <a href="/login/spa">SPA login</a></p>`
	if l.authenticated(r) {
		authLinks = `<p>You are authenticated. <a href="/app/dashboard">Dashboard</a> · <a href="/logout">Logout</a></p>`
	}
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>AuthLab Home</title></head>
<body>
  <h1>AuthLab</h1>
  %s
  <a href="/about-public">Public about</a>
</body></html>`, authLinks)
}

func (l *Lab) handleAboutPublic(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>About</title></head>
<body><h1>Public about</h1><a href="/">Home</a></body></html>`)
}

func (l *Lab) handleLogin(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Login</title></head>
<body>
  <h1>Sign in</h1>
  <form id="login-form" method="POST" action="/login">
    <label>Email <input id="email" name="email" type="email" aria-label="Email" /></label>
    <label>Password <input id="password" name="password" type="password" aria-label="Password" /></label>
    <button id="submit" type="submit" aria-label="Sign in">Sign in</button>
  </form>
</body></html>`)
	case http.MethodPost:
		l.LoginPosts.Add(1)
		_ = r.ParseForm()
		if r.FormValue("email") != Username || r.FormValue("password") != Password {
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}
		l.setSession(w)
		http.Redirect(w, r, "/app/dashboard", http.StatusFound)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLoginStep serves a username-first flow entirely client-side so a
// Chrome Recorder captures navigate → fill email → click next → wait password
// → fill password → click submit.
func (l *Lab) handleLoginStep(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Step Login</title></head>
<body>
  <h1>Step login</h1>
  <div id="panel-email">
    <label>Email <input id="email" type="email" aria-label="Email" /></label>
    <button id="next" type="button" aria-label="Next">Next</button>
  </div>
  <div id="panel-password" style="display:none">
    <label>Password <input id="password" type="password" aria-label="Password" /></label>
    <button id="submit" type="button" aria-label="Sign in">Sign in</button>
  </div>
  <p id="status"></p>
  <script>
    let email = "";
    document.getElementById("next").onclick = () => {
      email = document.getElementById("email").value;
      document.getElementById("panel-email").style.display = "none";
      document.getElementById("panel-password").style.display = "block";
    };
    document.getElementById("submit").onclick = async () => {
      const password = document.getElementById("password").value;
      const res = await fetch("/api/login", {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        credentials: "same-origin",
        body: JSON.stringify({email, password})
      });
      if (res.ok) { window.location = "/app/dashboard"; }
      else { document.getElementById("status").textContent = "login failed"; }
    };
  </script>
</body></html>`)
}

func (l *Lab) handleLoginSPA(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>SPA Login</title></head>
<body>
  <h1>SPA login</h1>
  <input id="email" type="email" aria-label="Email" placeholder="email" />
  <input id="password" type="password" aria-label="Password" placeholder="password" />
  <button id="submit" type="button" aria-label="Sign in">Sign in</button>
  <div id="app"></div>
  <script>
    document.getElementById("submit").onclick = async () => {
      const email = document.getElementById("email").value;
      const password = document.getElementById("password").value;
      const res = await fetch("/api/login", {
        method: "POST",
        headers: {"Content-Type": "application/json"},
        credentials: "same-origin",
        body: JSON.stringify({email, password})
      });
      if (!res.ok) { document.getElementById("app").textContent = "fail"; return; }
      window.location = "/app/dashboard";
    };
  </script>
</body></html>`)
}

func (l *Lab) handleAPILogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	l.LoginPosts.Add(1)
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if body.Email != Username || body.Password != Password {
		http.Error(w, `{"error":"invalid"}`, http.StatusUnauthorized)
		return
	}
	l.setSession(w)
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprint(w, `{"ok":true}`)
}

func (l *Lab) requireAuth(w http.ResponseWriter, r *http.Request) bool {
	if l.authenticated(r) {
		return true
	}
	http.Error(w, "unauthorized", http.StatusUnauthorized)
	return false
}

func (l *Lab) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if !l.requireAuth(w, r) {
		return
	}
	l.DashboardHits.Add(1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Dashboard</title></head>
<body>
  <h1>Dashboard</h1>
  <a href="/app/secret">Secret</a>
  <a href="/app/settings">Settings</a>
  <a href="/logout">Logout</a>
</body></html>`)
}

func (l *Lab) handleSecret(w http.ResponseWriter, r *http.Request) {
	if !l.requireAuth(w, r) {
		return
	}
	l.SecretHits.Add(1)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprintf(w, `<!DOCTYPE html>
<html><head><title>Secret</title></head>
<body>
  <h1>Secret</h1>
  <p id="marker">%s</p>
  <a href="/app/dashboard">Back</a>
</body></html>`, SecretMarker)
}

func (l *Lab) handleSettings(w http.ResponseWriter, r *http.Request) {
	if !l.requireAuth(w, r) {
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = fmt.Fprint(w, `<!DOCTYPE html>
<html><head><title>Settings</title></head>
<body>
  <h1>Settings</h1>
  <a href="/app/dashboard">Back</a>
</body></html>`)
}

func (l *Lab) handleLogout(w http.ResponseWriter, r *http.Request) {
	l.clearSession(w)
	http.Redirect(w, r, "/", http.StatusFound)
}

// ChromeRecordingSimple returns a Chrome DevTools Recorder export for the
// classic /login form.
func ChromeRecordingSimple(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf(`{
  "title": "authlab simple login",
  "steps": [
    {"type": "setViewport", "width": 1280, "height": 720},
    {"type": "navigate", "url": %q},
    {"type": "change", "value": %q, "selectors": [["#email"], ["aria/Email"]]},
    {"type": "change", "value": %q, "selectors": [["#password"], ["aria/Password"]]},
    {"type": "click", "selectors": [["#submit"], ["aria/Sign in"]]}
  ]
}`, baseURL+"/login", Username, Password)
}

// ChromeRecordingStep returns a Recorder export for the username-first flow.
func ChromeRecordingStep(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf(`{
  "title": "authlab step login",
  "steps": [
    {"type": "navigate", "url": %q},
    {"type": "change", "value": %q, "selectors": [["#email"]]},
    {"type": "click", "selectors": [["#next"], ["aria/Next"]]},
    {"type": "waitForElement", "selectors": [["#password"]]},
    {"type": "change", "value": %q, "selectors": [["#password"]]},
    {"type": "click", "selectors": [["#submit"], ["aria/Sign in"]]}
  ]
}`, baseURL+"/login/step", Username, Password)
}

// ChromeRecordingSPA returns a Recorder export for the SPA fetch login.
func ChromeRecordingSPA(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf(`{
  "title": "authlab spa login",
  "steps": [
    {"type": "navigate", "url": %q},
    {"type": "change", "value": %q, "selectors": [["#email"], ["aria/Email"]]},
    {"type": "change", "value": %q, "selectors": [["#password"], ["aria/Password"]]},
    {"type": "click", "selectors": [["#submit"], ["aria/Sign in"]]}
  ]
}`, baseURL+"/login/spa", Username, Password)
}

// ExplicitStepsSimple is a hand-authored step list for the classic login.
func ExplicitStepsSimple(baseURL string) string {
	baseURL = strings.TrimRight(baseURL, "/")
	return fmt.Sprintf(`{
  "steps": [
    {"action": "navigate", "value": %q},
    {"action": "fill", "selector": "#email", "value": "{{username}}"},
    {"action": "fill", "selector": "#password", "value": "{{password}}"},
    {"action": "click", "selector": "#submit"}
  ]
}`, baseURL+"/login")
}
