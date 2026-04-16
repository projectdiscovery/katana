package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-rod/rod"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth/recipe"
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
)

// Orchestrator manages the authentication lifecycle with a four-tier strategy:
//  1. Cached recipe replay (zero AI, zero heuristic)
//  2. Heuristic login (form detection + fill + verify)
//  3. AI agent login (if configured)
//  4. Browser handover (manual user login)
type Orchestrator struct {
	config      *AuthConfig
	recipeStore *recipe.Store
	loginAgent  LoginAgent
	showBrowser bool
	logger      *slog.Logger
}

// OrchestratorOption configures the Orchestrator.
type OrchestratorOption func(*Orchestrator)

// WithLoginAgent sets the AI login agent.
func WithLoginAgent(agent LoginAgent) OrchestratorOption {
	return func(o *Orchestrator) { o.loginAgent = agent }
}

// WithShowBrowser enables browser handover fallback.
func WithShowBrowser(show bool) OrchestratorOption {
	return func(o *Orchestrator) { o.showBrowser = show }
}

// WithLogger sets the structured logger.
func WithLogger(logger *slog.Logger) OrchestratorOption {
	return func(o *Orchestrator) { o.logger = logger }
}

// NewOrchestrator creates an auth orchestrator.
func NewOrchestrator(config *AuthConfig, opts ...OrchestratorOption) *Orchestrator {
	o := &Orchestrator{
		config: config,
		logger: slog.Default(),
	}
	for _, opt := range opts {
		opt(o)
	}

	// Best-effort recipe store init
	store, err := recipe.NewStore()
	if err != nil {
		o.logger.Warn("Recipe store unavailable", slog.String("error", err.Error()))
	} else {
		o.recipeStore = store
	}

	return o
}

// Authenticate performs the full login flow using the four-tier strategy.
// The page should already be navigated to the login URL or the target site.
func (o *Orchestrator) Authenticate(ctx context.Context, page *browser.BrowserPage) (*SessionState, error) {
	domain := o.extractDomain()

	o.logger.Info("[auth] Starting authentication", slog.String("domain", domain), slog.String("login_url", o.config.LoginURL))

	// === TIER 1: Cached Recipe ===
	if o.recipeStore != nil {
		session, err := o.tryRecipeReplay(ctx, page, domain)
		if err == nil {
			o.logger.Info("[auth] Login successful via cached recipe",
				slog.String("domain", domain),
				slog.Int("cookies", len(session.Cookies)),
			)
			return session, nil
		}
		if err != errNoRecipe {
			o.logger.Info("[auth] Cached recipe failed, trying heuristic", slog.String("reason", err.Error()))
		}
	}

	// Navigate to login URL
	if o.config.LoginURL != "" {
		o.logger.Info("[auth] Navigating to login page", slog.String("url", o.config.LoginURL))
		if err := page.Navigate(o.config.LoginURL); err != nil {
			return nil, fmt.Errorf("navigate to login: %w", err)
		}
		page.WaitPageLoadHeurisitics()
	}

	// Detect login page structure
	forms, _ := page.GetAllForms()
	detection := DetectLoginPage(page.Page, forms)

	if detection != nil {
		o.logger.Info("[auth] Login page analysis",
			slog.Bool("is_login", detection.IsLoginPage),
			slog.Float64("confidence", detection.Confidence),
			slog.Bool("has_password", detection.PasswordField != nil),
			slog.Bool("has_username", detection.UsernameField != nil),
			slog.Bool("has_submit", detection.SubmitButton != nil),
			slog.Any("oauth_providers", detection.OAuthProviders),
		)
	} else {
		o.logger.Info("[auth] No login form detected on page")
	}

	// === TIER 2: Heuristic Login ===
	if detection != nil && detection.IsLoginPage {
		o.logger.Info("[auth] Attempting heuristic login",
			slog.String("username_field", fieldSelector(detection.UsernameField)),
			slog.String("password_field", fieldSelector(detection.PasswordField)),
			slog.String("submit", fieldSelector(detection.SubmitButton)),
		)
		session, r, err := HeuristicLogin(page.Page, o.config, detection)
		if err == nil {
			o.saveRecipe(r, domain)
			o.logger.Info("[auth] Heuristic login successful",
				slog.String("domain", domain),
				slog.Int("cookies", len(session.Cookies)),
				slog.String("landed_at", r.Metadata.SuccessURL),
			)
			return session, nil
		}
		o.logger.Info("[auth] Heuristic login failed, trying next method", slog.String("reason", err.Error()))
	}

	// === TIER 3: AI Agent ===
	if o.loginAgent != nil {
		// Navigate back to login page to reset state after failed heuristic attempt
		if o.config.LoginURL != "" {
			o.logger.Info("[auth] Resetting page to login URL before AI agent attempt")
			_ = page.Navigate(o.config.LoginURL)
			page.WaitPageLoadHeurisitics()
		}
		o.logger.Info("[auth] Attempting AI agent login")
		toolkit := NewBrowserToolkit(page)

		// Wrap in recover to catch panics from the agent/API layer
		var agentResult *LoginResult
		var agentErr error
		func() {
			defer func() {
				if r := recover(); r != nil {
					agentErr = fmt.Errorf("agent panicked: %v", r)
					o.logger.Warn("[auth] AI agent panicked", slog.Any("panic", r))
				}
			}()
			agentResult, agentErr = o.loginAgent.Login(ctx, toolkit, o.config)
		}()

		if agentErr == nil && agentResult != nil && agentResult.Success {
			if agentResult.Recipe != nil {
				o.saveRecipe(agentResult.Recipe, domain)
			}
			o.logger.Info("[auth] AI agent login successful", slog.String("domain", domain))
			if agentResult.Session != nil {
				return agentResult.Session, nil
			}
			return ExtractSession(page.Page)
		}
		if agentErr != nil {
			o.logger.Info("[auth] AI agent login failed", slog.String("reason", agentErr.Error()))
		} else if agentResult != nil {
			o.logger.Info("[auth] AI agent login returned failure",
				slog.String("reason", agentResult.Error),
				slog.Bool("success", agentResult.Success),
			)
		} else {
			o.logger.Info("[auth] AI agent returned nil result")
		}
	}

	// === TIER 4: Browser Handover ===
	if o.showBrowser {
		o.logger.Info("[auth] Starting browser handover — please log in manually")
		session, err := BrowserHandover(page.Page, o.config, 5*time.Minute)
		if err == nil {
			o.logger.Info("[auth] Browser handover login successful",
				slog.String("domain", domain),
				slog.Int("cookies", len(session.Cookies)),
			)
			return session, nil
		}
		o.logger.Info("[auth] Browser handover failed", slog.String("reason", err.Error()))
	}

	return nil, fmt.Errorf("all login methods exhausted for %s", domain)
}

var errNoRecipe = fmt.Errorf("no cached recipe")

// tryRecipeReplay attempts to replay a cached login recipe.
func (o *Orchestrator) tryRecipeReplay(ctx context.Context, page *browser.BrowserPage, domain string) (*SessionState, error) {
	cached, err := o.recipeStore.Get(domain)
	if err != nil {
		return nil, errNoRecipe
	}

	o.logger.Debug("Found cached recipe", slog.String("domain", domain), slog.Int("version", cached.Version))

	// Navigate to the login page
	if cached.LoginURL != "" {
		if err := page.Navigate(cached.LoginURL); err != nil {
			return nil, fmt.Errorf("navigate: %w", err)
		}
		page.WaitPageLoadHeurisitics()
	}

	// Validate that recipe selectors still exist on the page
	if !recipe.ValidateRecipe(page.Page, cached) {
		return nil, fmt.Errorf("recipe selectors stale")
	}

	// Replay the recipe
	if err := recipe.ReplayRecipe(page.Page, cached, o.config.Credentials.GetUsername(), o.config.Credentials.Password); err != nil {
		return nil, fmt.Errorf("replay: %w", err)
	}

	// Wait for navigation after replay
	waitForNavigation(page.Page, cached.LoginURL, 5*time.Second)

	// Verify login succeeded
	session, err := ExtractSession(page.Page)
	if err != nil {
		return nil, fmt.Errorf("extract session: %w", err)
	}

	// Check we actually have session cookies
	if len(session.Cookies) == 0 {
		return nil, fmt.Errorf("no cookies after replay")
	}

	// Bump usage count
	cached.UsedCount++
	o.recipeStore.Save(cached)

	return session, nil
}

func (o *Orchestrator) saveRecipe(r *recipe.Recipe, domain string) {
	if o.recipeStore == nil || r == nil {
		return
	}
	r.Domain = domain
	if err := o.recipeStore.Save(r); err != nil {
		o.logger.Warn("Failed to save recipe", slog.String("error", err.Error()))
	}
}

func fieldSelector(f *FieldInfo) string {
	if f == nil {
		return "(not found)"
	}
	return f.Selector
}

func (o *Orchestrator) extractDomain() string {
	if o.config.LoginURL != "" {
		if u, err := url.Parse(o.config.LoginURL); err == nil {
			return u.Hostname()
		}
	}
	return "unknown"
}

// BrowserHandover shows the browser to the user for manual login.
// It polls for success signals (new cookies, URL change) until timeout.
func BrowserHandover(page *rod.Page, config *AuthConfig, timeout time.Duration) (*SessionState, error) {
	if config.LoginURL != "" {
		if err := page.Navigate(config.LoginURL); err != nil {
			return nil, fmt.Errorf("handover navigate: %w", err)
		}
		// Short wait for the login page to render
		time.Sleep(2 * time.Second)
	}

	fmt.Println("[AUTH] Browser handover: please log in via the browser window")
	fmt.Printf("[AUTH] Waiting up to %s for authentication...\n", timeout)

	beforeCookies, _ := GetCookiesFromPage(page)

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)

		// Check if cookies changed (new session cookie appeared)
		afterCookies, _ := GetCookiesFromPage(page)
		diff := DiffCookies(beforeCookies, afterCookies)
		if len(diff) > 0 {
			// Check we're not still on the login page
			info, _ := page.Info()
			if info != nil && config.LoginURL != "" {
				loginPath := extractPath(config.LoginURL)
				currentPath := extractPath(info.URL)
				if loginPath != currentPath {
					fmt.Println("[AUTH] Authentication detected — resuming crawl")
					return ExtractSession(page)
				}
			} else {
				fmt.Println("[AUTH] Authentication detected — resuming crawl")
				return ExtractSession(page)
			}
		}
	}

	return nil, fmt.Errorf("browser handover timed out after %s", timeout)
}

// SessionMonitor periodically checks if the authenticated session is still valid.
// It runs heuristic checks (cookie presence, login redirect detection, logout element visibility)
// and re-authenticates if the session expires.
type SessionMonitor struct {
	orchestrator *Orchestrator
	session      *SessionState
	recipeMeta   *recipe.Metadata
	checkEvery   int
	navCount     atomic.Int64
	mu           sync.RWMutex
	logger       *slog.Logger
}

// NewSessionMonitor creates a session health monitor.
func NewSessionMonitor(orch *Orchestrator, session *SessionState, meta *recipe.Metadata, logger *slog.Logger) *SessionMonitor {
	return &SessionMonitor{
		orchestrator: orch,
		session:      session,
		recipeMeta:   meta,
		checkEvery:   20,
		logger:       logger,
	}
}

// IncrementNav records a navigation event.
func (m *SessionMonitor) IncrementNav() {
	m.navCount.Add(1)
}

// ShouldCheck returns true if it's time for a session health check.
func (m *SessionMonitor) ShouldCheck() bool {
	count := m.navCount.Load()
	return count > 0 && count%int64(m.checkEvery) == 0
}

// Check verifies that the authenticated session is still valid.
func (m *SessionMonitor) Check(page *rod.Page) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Check 1: Session cookie still present
	if m.recipeMeta != nil && m.recipeMeta.SessionCookie != "" {
		cookies, err := GetCookiesFromPage(page)
		if err != nil {
			return true // can't check — assume OK
		}
		found := false
		for _, c := range cookies {
			if c.Name == m.recipeMeta.SessionCookie && c.Value != "" {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Check 2: Not redirected to login page
	info, err := page.Info()
	if err == nil && info != nil {
		if IsLoginURL(info.URL) {
			return false
		}
	}

	// Check 3: Logout element still visible (from recipe metadata)
	if m.recipeMeta != nil && m.recipeMeta.LogoutSelector != "" {
		el, err := page.Element(m.recipeMeta.LogoutSelector)
		if err == nil && el != nil {
			visible, _ := el.Visible()
			if visible {
				return true // definitively logged in
			}
		}
	}

	return true // no negative signals
}

// ReAuthenticate triggers a new login and updates the stored session.
func (m *SessionMonitor) ReAuthenticate(ctx context.Context, page *browser.BrowserPage) error {
	newSession, err := m.orchestrator.Authenticate(ctx, page)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.session = newSession
	m.mu.Unlock()
	return nil
}

// CurrentSession returns the current session state (thread-safe).
func (m *SessionMonitor) CurrentSession() *SessionState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.session
}
