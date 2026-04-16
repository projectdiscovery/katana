package headless

import (
	"context"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lmittmann/tint"
	"github.com/projectdiscovery/gologger"
	"github.com/projectdiscovery/katana/pkg/engine/headless/auth"
	_ "github.com/projectdiscovery/katana/pkg/engine/headless/auth/agentlogin" // registers "anthropic" login agent
	"github.com/projectdiscovery/katana/pkg/engine/headless/browser"
	"github.com/projectdiscovery/katana/pkg/engine/headless/captcha"
	"github.com/projectdiscovery/katana/pkg/engine/headless/explorer"
	_ "github.com/projectdiscovery/katana/pkg/engine/headless/captcha/capsolver"
	"github.com/projectdiscovery/katana/pkg/engine/headless/crawler"
	"github.com/projectdiscovery/katana/pkg/engine/headless/netlog"
	"github.com/projectdiscovery/katana/pkg/engine/headless/report"
	"github.com/projectdiscovery/katana/pkg/engine/parser"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/secrets"
	"github.com/projectdiscovery/katana/pkg/types"
	"github.com/projectdiscovery/katana/pkg/utils"
)

type Headless struct {
	logger  *slog.Logger
	options *types.CrawlerOptions

	pathTrie *utils.PathTrie

	debugger *CrawlDebugger
}

// New returns a new headless crawler instance
func New(options *types.CrawlerOptions) (*Headless, error) {
	logger := newLogger(options)

	headless := &Headless{
		logger:  logger,
		options: options,
	}
	if options.Options.FilterSimilar {
		headless.pathTrie = utils.NewPathTrie(options.Options.FilterSimilarThreshold)
	}

	// Show crawl debugger if verbose is enabled
	if options.Options.Verbose {
		headless.debugger = NewCrawlDebugger(8089)
	}

	return headless, nil
}

func newLogger(options *types.CrawlerOptions) *slog.Logger {
	if options.Logger != nil {
		return options.Logger
	}

	writer := os.Stderr

	// set global logger with custom options
	level := slog.LevelInfo
	if options.Options.Debug {
		level = slog.LevelDebug
	}
	logger := slog.New(
		tint.NewHandler(writer, &tint.Options{
			Level:      level,
			TimeFormat: time.Kitchen,
		}),
	)
	return logger
}

func validateScopeFunc(h *Headless, URL string) browser.ScopeValidator {
	parsedURL, err := url.Parse(URL)
	if err != nil {
		return func(string) bool { return true }
	}
	rootHostname := parsedURL.Hostname()

	return func(s string) bool {
		if h.options.ScopeManager == nil {
			return true
		}
		parsed, err := url.Parse(s)
		if err != nil {
			return false
		}
		validated, err := h.options.ScopeManager.Validate(parsed, rootHostname)
		if err != nil {
			return false
		}
		return validated
	}
}

// Crawl executes the headless crawling on a given URL
func (h *Headless) Crawl(URL string) error {
	if h.debugger != nil {
		h.debugger.StartURL(URL, 0)
	}
	defer func() {
		if h.debugger != nil {
			h.debugger.EndURL(URL)
		}
	}()

	scopeValidator := validateScopeFunc(h, URL)

	// Report collector — captures results for attack surface report generation
	var collector *report.Collector
	if h.options.Options.Report {
		collector = report.NewCollector(URL)
	}

	// Async secrets scanning — goroutines scan response bodies in background
	// so they don't block the crawl loop. We wait for all scans before report generation.
	var secretsWg sync.WaitGroup
	// Dedup secrets logging: track rule+URL pairs we've already logged to avoid spam
	// (e.g. the same "LinkedIn Access Token" rule matching 14 times in one JS bundle).
	var secretsLogMu sync.Mutex
	secretsLogged := make(map[string]bool)

	// Network event sink — captures full request/response for HAR output and/or
	// network logging service. Created before the callback so it's available during crawl.
	eventSink := netlog.NewEventSink(h.options.Options.HAROutput, h.logger)
	if eventSink != nil {
		defer func() {
			if err := eventSink.Close(); err != nil {
				h.logger.Warn("[netlog] failed to close event sink", slog.String("error", err.Error()))
			}
		}()
	}

	// Request yield counter — tracks new unique URLs seen by the RequestCallback.
	// The crawler reads the delta before/after action execution to detect API calls
	// triggered by page visits (used for smarter exhaustion decisions).
	requestYieldCounter := &atomic.Int64{}

	crawlOpts := crawler.Options{
		ChromiumPath:      h.options.Options.SystemChromePath,
		MaxDepth:          h.options.Options.MaxDepth,
		ShowBrowser:       h.options.Options.ShowBrowser,
		MaxCrawlDuration:  h.options.Options.CrawlDuration,
		MaxFailureCount:   h.options.Options.MaxFailureCount,
		NoSandbox:         h.options.Options.HeadlessNoSandbox,
		NoIncognito:       h.options.Options.HeadlessNoIncognito,
		UserDataDir:       h.options.Options.ChromeDataDir,
		Proxy:             h.options.Options.Proxy,
		MaxBrowsers:       max(h.options.Options.HeadlessConcurrency, 1),
		PageMaxTimeout:    30 * time.Second,
		ScopeValidator:    scopeValidator,
		AutomaticFormFill: h.options.Options.AutomaticFormFill,
		PageLoadStrategy:  h.options.Options.PageLoadStrategy,
		ChromeWSUrl:       h.options.Options.ChromeWSUrl,
		DOMWaitTime:       h.options.Options.DOMWaitTime,
		CoverageGuided:      h.options.Options.CoverageGuided,
		RequestYieldCounter: requestYieldCounter,
		RequestCallback: func(rr *output.Result) {
			if rr == nil || rr.Request == nil {
				return
			}
			if scopeValidator != nil && !scopeValidator(rr.Request.URL) {
				return
			}

			// Async secrets scanning — copy body and scan in background goroutine
			// so we don't block the crawl loop. Findings go directly to the collector.
			if h.options.SecretsScanner != nil && rr.Response != nil && rr.Response.Body != "" {
				// Capture what we need before body is cleared later
				bodySnapshot := rr.Response.Body
				headerSnapshot := make(map[string]string, len(rr.Response.Headers))
				for k, v := range rr.Response.Headers {
					headerSnapshot[k] = v
				}
				reqURL := rr.Request.URL

				secretsWg.Add(1)
				go func() {
					defer secretsWg.Done()

					var findings []secrets.Finding
					findings = append(findings, h.options.SecretsScanner.ScanHeaders(headerSnapshot, "response_header", reqURL)...)
					if len(bodySnapshot) <= 10*1024*1024 {
						findings = append(findings, h.options.SecretsScanner.ScanString(bodySnapshot, "response_body", reqURL)...)
					}
					if len(findings) == 0 {
						return
					}

					// Log each unique rule+URL pair once to avoid spam when the
					// same rule fires many times in a single response body.
					secretsLogMu.Lock()
					for _, f := range findings {
						logKey := f.RuleName + "|" + reqURL
						if secretsLogged[logKey] {
							continue
						}
						secretsLogged[logKey] = true
						status := ""
						if f.Validation != nil {
							status = f.Validation.Status
						}
						h.logger.Info("[secrets] found",
							slog.String("rule", f.RuleName),
							slog.String("source", f.Source),
							slog.String("url", reqURL),
							slog.String("validation", status),
						)
					}
					secretsLogMu.Unlock()

					// Feed findings to report collector
					if collector != nil {
						collector.AddSecrets(reqURL, findings)
					}
				}()
			}

			// Collect result for report generation (before dedup so we count accurately)
			if collector != nil {
				collector.Collect(rr)
			}

			// Register the real (intercepted) request URL before parsing the
			// response body for additional discoveries. This ensures that real
			// results with full response data always take priority over
			// synthetic Request-only entries produced by performAdditionalAnalysis.
			if !h.isUniqueURL(rr.Request.URL) {
				return
			}
			requestYieldCounter.Add(1)

			// Run additional analysis (jsluice, URL extraction) only on unique URLs.
			// With multi-tab crawling, the same JS file may be fetched by multiple tabs —
			// analyzing it once is sufficient since discoveries are URL-deduplicated anyway.
			navigationRequests := h.performAdditionalAnalysis(rr, collector)
			for _, req := range navigationRequests {
				if err := h.options.OutputWriter.Write(req); err != nil {
					h.logger.Debug("failed to write navigation result",
						slog.String("url", func() string {
							if req != nil && req.Request != nil {
								return req.Request.URL
							}
							return ""
						}()),
						slog.String("error", err.Error()),
					)
				}
			}

			// Record to HAR / network logger while body is still available
			if eventSink != nil {
				eventSink.Record(rr, "crawl", fmt.Sprintf("depth:%d", rr.Request.Depth))
			}

			if rr.Response != nil {
				rr.Response.KnowledgeBase = h.options.ClassifyPage(rr.Response.Body)
				rr.Response.Raw = ""
				rr.Response.Body = ""
			}
			if err := h.options.OutputWriter.Write(rr); err != nil {
				h.logger.Debug("failed to write result",
					slog.String("error", err.Error()),
				)
			}
		},
		Logger:              h.logger,
		ChromeUser:          h.options.ChromeUser,
		EnableDiagnostics:   h.options.Options.EnableDiagnostics,
		Trace:               h.options.Options.EnableDiagnostics,
		CookieConsentBypass: true,
	}

	if provider := h.options.Options.CaptchaSolverProvider; provider != "" {
		gologger.Debug().Msgf("captcha solver enabled: provider=%s", provider)
		handler, err := captcha.NewHandler(provider, h.options.Options.CaptchaSolverAPIKey)
		if err != nil {
			gologger.Warning().Msgf("captcha handler init failed: %s", err)
		} else {
			crawlOpts.CaptchaHandler = handler
		}
	}

	// When auth is configured, force NoIncognito so session cookies persist
	// in the browser context. Incognito mode isolates cookies and breaks
	// authenticated crawling.
	var authConfig *auth.AuthConfig
	var authOrch *auth.Orchestrator
	if h.options.Options.AuthConfig != "" {
		var err error
		authConfig, err = auth.LoadAuthConfig(h.options.Options.AuthConfig)
		if err != nil {
			return fmt.Errorf("auth config: %w", err)
		}

		crawlOpts.NoIncognito = true
		h.logger.Info("[auth] Incognito disabled for authenticated crawling")

		// Resolve AI login agent — explicit flag takes priority, then auto-detect from env
		var loginAgent auth.LoginAgent
		provider := h.options.Options.AuthAIProvider
		apiKey := h.options.Options.AuthAIAPIKey

		// Auto-detect: if no provider specified but ANTHROPIC_API_KEY is in env, use anthropic
		if provider == "" && apiKey == "" {
			if envKey := os.Getenv("ANTHROPIC_API_KEY"); envKey != "" {
				provider = "anthropic"
				apiKey = envKey
				h.logger.Info("[auth] Auto-detected Anthropic API key from ANTHROPIC_API_KEY env var")
			}
		}
		if provider != "" && apiKey == "" {
			apiKey = os.Getenv("ANTHROPIC_API_KEY")
		}

		if provider != "" && apiKey != "" {
			loginAgent, err = auth.NewLoginAgent(provider, apiKey)
			if err != nil {
				h.logger.Warn("[auth] AI login agent init failed", slog.String("error", err.Error()))
			} else {
				h.logger.Info("[auth] AI login agent enabled as fallback", slog.String("provider", provider))
			}
		}

		authOrch = auth.NewOrchestrator(authConfig,
			auth.WithLoginAgent(loginAgent),
			auth.WithShowBrowser(h.options.Options.ShowBrowser),
			auth.WithLogger(h.logger),
		)

		crawlOpts.AuthConfig = authConfig
		crawlOpts.AuthOrchestrator = authOrch
	}

	// AI Exploration Planner — when enabled, the AI analyzes each page
	// and produces a prioritized exploration plan (forms to fill, menus to expand, etc.)
	if h.options.Options.AIPlanner {
		explorationPlanner := explorer.NewPlanner(h.logger)
		if explorationPlanner.HasAPIKey() {
			crawlOpts.ExplorationPlanner = explorationPlanner
			h.logger.Info("[explorer] AI page exploration planner enabled")
		} else {
			gologger.Warning().Msg("[explorer] --ai-planner enabled but no ANTHROPIC_API_KEY found")
		}
	}

	headlessCrawler, err := crawler.New(crawlOpts)
	if err != nil {
		return err
	}
	defer headlessCrawler.Close()

	// Authentication phase — authenticate on the SAME browser the crawler uses.
	// This way cookies persist in the browser context without needing CDP cookie transfer.
	if authConfig != nil && authConfig.LoginURL != "" {
		authPage, err := headlessCrawler.GetPageFromLauncher()
		if err != nil {
			return fmt.Errorf("auth page: %w", err)
		}

		session, authErr := authOrch.Authenticate(context.Background(), authPage)
		headlessCrawler.ReturnPageToLauncher(authPage)

		if authErr != nil {
			gologger.Warning().Msgf("Authentication failed: %s", authErr)
		} else {
			// Store session for monitoring, but cookies are already in the browser
			crawlOpts.SessionState = session
			h.logger.Info("[auth] Authentication successful",
				slog.String("url", authConfig.LoginURL),
				slog.Int("cookies", len(session.Cookies)),
			)

			// Feed auth info to report collector
			if collector != nil {
				authInfo := report.AuthInfo{
					Authenticated: true,
					LoginURL:      authConfig.LoginURL,
					Username:      authConfig.Credentials.GetUsername(),
					Method:        "form",
				}
				// Catalog session cookies (names only, not values)
				for _, c := range session.Cookies {
					authInfo.SessionCookies = append(authInfo.SessionCookies, report.SessionCookieInfo{
						Name:     c.Name,
						Domain:   c.Domain,
						Path:     c.Path,
						HTTPOnly: c.HTTPOnly,
						Secure:   c.Secure,
						SameSite: c.SameSite,
					})
				}
				if len(session.AuthHeaders) > 0 {
					authInfo.Method = "token"
					for k := range session.AuthHeaders {
						authInfo.AuthHeaderNames = append(authInfo.AuthHeaderNames, k)
					}
				}
				collector.SetAuthInfo(authInfo)
			}
		}
	}

	if err = headlessCrawler.Crawl(URL); err != nil {
		return err
	}

	// Wait for all async secrets scans to complete before report generation
	secretsWg.Wait()

	// Generate attack surface report after crawl completes
	if collector != nil {
		siteReport := collector.GenerateReport()

		// Add graph stats and visualization data
		if g := headlessCrawler.GetCrawlGraph(); g != nil {
			vertices := g.GetVertices()
			maxDepth := 0
			for _, vid := range vertices {
				if ps, err := g.GetPageState(vid); err == nil && ps.Depth > maxDepth {
					maxDepth = ps.Depth
				}
			}
			siteReport.GraphStats = &report.GraphStats{
				TotalPages: len(vertices),
				MaxDepth:   maxDepth,
				TotalEdges: g.EdgeCount(),
				TotalForms: len(siteReport.Forms),
			}
			siteReport.GraphNodes, siteReport.GraphEdges = g.ExportGraph()
		}

		if err := h.writeReport(siteReport); err != nil {
			h.logger.Warn("Failed to write report", slog.String("error", err.Error()))
		} else {
			dest := h.options.Options.ReportOutput
			if dest == "" {
				dest = "stdout"
			}
			h.logger.Info("[report] written",
				slog.String("format", h.options.Options.ReportFormat),
				slog.String("output", dest),
				slog.Int("endpoints", len(siteReport.Endpoints)),
				slog.Int("secrets", len(siteReport.Secrets)),
			)
		}
	}

	return nil
}

// writeReport renders the attack surface report in the configured format.
func (h *Headless) writeReport(siteReport *report.SiteReport) error {
	format := h.options.Options.ReportFormat
	if format == "" {
		format = "json"
	}

	// Determine output destination
	var w *os.File
	if outputPath := h.options.Options.ReportOutput; outputPath != "" {
		f, err := os.Create(outputPath)
		if err != nil {
			return fmt.Errorf("create report file: %w", err)
		}
		defer f.Close()
		w = f
	} else {
		w = os.Stdout
	}

	switch format {
	case "json":
		return report.RenderJSON(w, siteReport)
	case "markdown", "md":
		return report.RenderMarkdown(w, siteReport)
	default:
		return fmt.Errorf("unknown report format: %s (supported: json, markdown)", format)
	}
}

func (h *Headless) Close() error {
	if h.debugger != nil {
		h.debugger.Close()
	}
	return nil
}

func (h *Headless) isUniqueURL(rawURL string) bool {
	dedupKey := rawURL
	if h.options.Options.IgnoreQueryParams {
		dedupKey = utils.ReplaceAllQueryParam(dedupKey, "")
	}
	if h.options.Options.FilterSimilar {
		dedupKey = utils.FingerprintURL(dedupKey, h.pathTrie)
	}
	return h.options.UniqueFilter.UniqueURL(dedupKey)
}

func (h *Headless) performAdditionalAnalysis(rr *output.Result, collector *report.Collector) []*output.Result {
	responseParser := parser.NewResponseParser()
	newNavigations := responseParser.ParseResponse(rr.Response)

	// Enhanced analysis: run full jsluice extractors on JS responses
	// to discover routes, framework state, and additional endpoints.
	// Analysis runs on actual browser responses so it works behind
	// auth and respects scope — no standalone HTTP requests needed.
	if rr.Response != nil && rr.Response.Body != "" && isJavaScriptResponse(rr) {
		analysis := utils.FullJSLuiceAnalysis(rr.Response.Body)
		if analysis != nil {
			sourceURL := rr.Request.URL

			// Log findings
			if len(analysis.Frameworks) > 0 {
				names := make([]string, 0, len(analysis.Frameworks))
				for _, fw := range analysis.Frameworks {
					name := fw.Name
					if fw.Version != "" {
						name += " " + fw.Version
					}
					names = append(names, name)
				}
				h.logger.Info("[jsluice] frameworks detected",
					slog.String("source", sourceURL),
					slog.String("frameworks", strings.Join(names, ", ")),
				)
			}
			if len(analysis.Routes) > 0 {
				h.logger.Info("[jsluice] routes extracted",
					slog.String("source", sourceURL),
					slog.Int("count", len(analysis.Routes)),
				)
			}
			if len(analysis.FrameworkState) > 0 {
				for _, state := range analysis.FrameworkState {
					h.logger.Info("[jsluice] framework state found",
						slog.String("source", sourceURL),
						slog.String("kind", state.Kind),
						slog.String("key", state.Key),
						slog.Int("urls", len(state.APIURLs)),
					)
				}
			}
			if len(analysis.SourceMapURLs) > 0 {
				for _, sm := range analysis.SourceMapURLs {
					h.logger.Warn("[jsluice] source map exposed (information disclosure)",
						slog.String("source", sourceURL),
						slog.String("map", sm),
					)
				}
			}
			if len(analysis.PostMessage) > 0 {
				for _, pm := range analysis.PostMessage {
					if string(pm.Severity) == "high" || string(pm.Severity) == "medium" {
						h.logger.Warn("[jsluice] postMessage risk",
							slog.String("source", sourceURL),
							slog.String("type", pm.Type),
							slog.String("severity", string(pm.Severity)),
							slog.Bool("origin_check", pm.HasOriginCheck),
						)
					}
				}
			}
			if len(analysis.GraphQLOps) > 0 {
				names := make([]string, 0, len(analysis.GraphQLOps))
				for _, op := range analysis.GraphQLOps {
					names = append(names, op.Type+" "+op.Name)
				}
				h.logger.Info("[jsluice] GraphQL operations found",
					slog.String("source", sourceURL),
					slog.String("operations", strings.Join(names, ", ")),
				)
			}
			if len(analysis.Secrets) > 0 {
				for _, secret := range analysis.Secrets {
					h.logger.Warn("[jsluice] secret found in JS bundle",
						slog.String("source", sourceURL),
						slog.String("kind", secret.Kind),
						slog.String("severity", string(secret.Severity)),
					)
				}
			}

			// Routes discovered in JS bundles → navigation requests
			for _, route := range analysis.Routes {
				if route.Path != "" {
					newNavigations = append(newNavigations, &navigation.Request{
						Method:    "GET",
						URL:       rr.Response.AbsoluteURL(route.Path),
						Source:    sourceURL,
						Tag:       "js",
						Attribute: "jsluice-route-" + route.Framework,
					})
				}
			}

			// Framework state embedded URLs → navigation requests
			for _, state := range analysis.FrameworkState {
				for _, apiURL := range state.APIURLs {
					newNavigations = append(newNavigations, &navigation.Request{
						Method:    "GET",
						URL:       rr.Response.AbsoluteURL(apiURL),
						Source:    sourceURL,
						Tag:       "js",
						Attribute: "jsluice-state-" + state.Kind,
					})
				}
			}

			// GraphQL operations → infer /graphql endpoint
			if len(analysis.GraphQLOps) > 0 {
				newNavigations = append(newNavigations, &navigation.Request{
					Method:    "GET",
					URL:       rr.Response.AbsoluteURL("/graphql"),
					Source:    sourceURL,
					Tag:       "js",
					Attribute: "jsluice-graphql",
				})
			}

			// Feed all findings to report collector
			if collector != nil {
				feedAnalysisToCollector(analysis, collector)
			}
		}
	}

	navigationRequests := make([]*output.Result, 0)
	for _, resp := range newNavigations {
		if !h.isUniqueURL(resp.URL) {
			continue
		}
		navigationRequests = append(navigationRequests, &output.Result{
			Request: resp,
		})
	}
	return navigationRequests
}

// feedAnalysisToCollector pushes jsluice findings into the report collector.
func feedAnalysisToCollector(analysis *utils.JSLuiceAnalysisResult, collector *report.Collector) {
	// Frameworks
	var fwInfos []report.FrameworkInfo
	for _, fw := range analysis.Frameworks {
		fwInfos = append(fwInfos, report.FrameworkInfo{
			Name:     fw.Name,
			Version:  fw.Version,
			Evidence: fw.Evidence,
		})
	}
	if len(fwInfos) > 0 {
		collector.AddFrameworks(fwInfos)
	}

	// Routes
	var routeInfos []report.RouteInfo
	for _, r := range analysis.Routes {
		routeInfos = append(routeInfos, report.RouteInfo{
			Path:       r.Path,
			Method:     r.Method,
			Framework:  r.Framework,
			ParamNames: r.ParamNames,
		})
	}
	if len(routeInfos) > 0 {
		collector.AddRoutes(routeInfos)
	}

	// Source maps
	if len(analysis.SourceMapURLs) > 0 {
		collector.AddSourceMaps(analysis.SourceMapURLs)
	}

	// postMessage findings
	var pmFindings []report.PostMessageFinding
	for _, pm := range analysis.PostMessage {
		pmFindings = append(pmFindings, report.PostMessageFinding{
			Type:           pm.Type,
			HandlerType:    pm.HandlerType,
			HasOriginCheck: pm.HasOriginCheck,
			AllowedOrigins: pm.AllowedOrigins,
			TargetOrigin:   pm.TargetOrigin,
			DataSinks:      pm.DataSinks,
			Severity:       string(pm.Severity),
			Source:         pm.Source,
			Filename:       pm.Filename,
		})
	}
	if len(pmFindings) > 0 {
		collector.AddPostMessageFindings(pmFindings)
	}

	// GraphQL operations
	var gqlOps []report.GraphQLOpInfo
	for _, gql := range analysis.GraphQLOps {
		gqlOps = append(gqlOps, report.GraphQLOpInfo{
			Type: gql.Type,
			Name: gql.Name,
		})
	}
	if len(gqlOps) > 0 {
		collector.AddGraphQLOps(gqlOps)
	}
}

// isJavaScriptResponse checks if a response contains JavaScript content.
func isJavaScriptResponse(rr *output.Result) bool {
	if rr.Response == nil {
		return false
	}
	ct := ""
	if h, ok := rr.Response.Headers["content-type"]; ok {
		ct = h
	}
	if strings.Contains(ct, "javascript") || strings.Contains(ct, "ecmascript") {
		return true
	}
	// Check URL extension
	if rr.Request != nil {
		u := rr.Request.URL
		if strings.HasSuffix(u, ".js") || strings.HasSuffix(u, ".mjs") {
			return true
		}
	}
	return false
}
