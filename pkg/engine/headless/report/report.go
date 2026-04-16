package report

import (
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/projectdiscovery/katana/pkg/engine/headless/graph"
	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/output"
	"github.com/projectdiscovery/katana/pkg/secrets"
)

// SiteReport is the complete attack surface report produced after a crawl.
type SiteReport struct {
	// Metadata
	TargetURL     string        `json:"target_url"`
	CrawlTime     time.Time     `json:"crawl_time"`
	CrawlDuration time.Duration `json:"crawl_duration"`
	TotalRequests int           `json:"total_requests"`

	// Authentication
	Auth *AuthInfo `json:"auth,omitempty"`

	// Attack surface by security category
	AttackSurface AttackSurface `json:"attack_surface"`

	// Technical details
	Technologies    []string        `json:"technologies,omitempty"`
	Forms           []FormInfo      `json:"forms,omitempty"`
	SecurityHeaders []HeaderFinding `json:"security_headers,omitempty"`
	Secrets         []SecretFinding `json:"secrets,omitempty"`

	// All discovered endpoints (deduplicated)
	Endpoints []Endpoint `json:"endpoints"`

	// Static analysis findings (Phase 0)
	Frameworks       []FrameworkInfo       `json:"frameworks,omitempty"`
	Routes           []RouteInfo           `json:"routes,omitempty"`
	SourceMaps       []string              `json:"source_maps,omitempty"`
	PostMessageRisks []PostMessageFinding  `json:"postmessage_risks,omitempty"`
	GraphQLOps       []GraphQLOpInfo       `json:"graphql_operations,omitempty"`

	// Graph visualization
	GraphStats *GraphStats       `json:"graph_stats,omitempty"`
	GraphNodes []graph.GraphNode `json:"graph_nodes,omitempty"`
	GraphEdges []graph.GraphEdge `json:"graph_edges,omitempty"`
}

// FrameworkInfo describes a detected frontend framework.
type FrameworkInfo struct {
	Name     string   `json:"name"`
	Version  string   `json:"version,omitempty"`
	Evidence []string `json:"evidence,omitempty"`
}

// RouteInfo describes a discovered route definition.
type RouteInfo struct {
	Path       string   `json:"path"`
	Method     string   `json:"method,omitempty"`
	Framework  string   `json:"framework"`
	ParamNames []string `json:"param_names,omitempty"`
	Source     string   `json:"source,omitempty"`
}

// PostMessageFinding describes a postMessage security finding.
type PostMessageFinding struct {
	Type           string   `json:"type"`
	HandlerType    string   `json:"handler_type,omitempty"`
	HasOriginCheck bool     `json:"has_origin_check"`
	AllowedOrigins []string `json:"allowed_origins,omitempty"`
	TargetOrigin   string   `json:"target_origin,omitempty"`
	DataSinks      []string `json:"data_sinks,omitempty"`
	Severity       string   `json:"severity"`
	Source         string   `json:"source,omitempty"`
	Filename       string   `json:"filename,omitempty"`
}

// GraphQLOpInfo describes a discovered GraphQL operation.
type GraphQLOpInfo struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// AttackSurface classifies endpoints by security interest.
type AttackSurface struct {
	AuthEndpoints   []Endpoint `json:"auth_endpoints,omitempty"`
	AdminEndpoints  []Endpoint `json:"admin_endpoints,omitempty"`
	FileEndpoints   []Endpoint `json:"file_endpoints,omitempty"`
	SearchEndpoints []Endpoint `json:"search_endpoints,omitempty"`
	DataMutation    []Endpoint `json:"data_mutation,omitempty"`
	UserSpecific    []Endpoint `json:"user_specific,omitempty"`
	APIEndpoints    []Endpoint `json:"api_endpoints,omitempty"`
	ExternalInputs  []Endpoint `json:"external_inputs,omitempty"`
	Parameterized   []Endpoint `json:"parameterized,omitempty"`   // Endpoints with query params (injection surface)
	HasForms        []Endpoint `json:"has_forms,omitempty"`       // Pages containing forms
	Uncategorized   []Endpoint `json:"uncategorized,omitempty"`   // Endpoints not matching any pattern

	Summary AttackSurfaceSummary `json:"summary"`
}

// AttackSurfaceSummary provides counts for quick assessment.
type AttackSurfaceSummary struct {
	TotalEndpoints  int            `json:"total_endpoints"`
	ByCategory      map[string]int `json:"by_category"`
	ByMethod        map[string]int `json:"by_method"`
	ByStatusCode    map[int]int    `json:"by_status_code"`
	UniqueHosts     int            `json:"unique_hosts"`
	WithParams      int            `json:"with_params"`
	WithForms       int            `json:"with_forms"`
	Uncategorized   int            `json:"uncategorized"`
	SecretsFound    int            `json:"secrets_found"`
}

// Endpoint represents a single discovered endpoint with security classification.
type Endpoint struct {
	URL          string     `json:"url"`
	Method       string     `json:"method"`
	StatusCode   int        `json:"status_code,omitempty"`
	Categories   []string   `json:"categories"`
	Source       string     `json:"source,omitempty"`
	Depth        int        `json:"depth,omitempty"`
	HasParams    bool       `json:"has_params,omitempty"`
	ParamNames   []string   `json:"param_names,omitempty"`
	FormCount    int        `json:"form_count,omitempty"`
	Technologies []string   `json:"technologies,omitempty"`
	ContentType  string     `json:"content_type,omitempty"`
	Title        string     `json:"title,omitempty"`
}

// FormInfo describes a discovered HTML form.
type FormInfo struct {
	Action     string   `json:"action"`
	Method     string   `json:"method"`
	Parameters []string `json:"parameters,omitempty"`
	Enctype    string   `json:"enctype,omitempty"`
	PageURL    string   `json:"page_url,omitempty"`
}

// HeaderFinding records a missing or misconfigured security header.
type HeaderFinding struct {
	URL     string `json:"url"`
	Header  string `json:"header"`
	Status  string `json:"status"` // "missing", "weak", "present"
	Value   string `json:"value,omitempty"`
}

// SecretFinding records a secret detected in HTTP traffic.
type SecretFinding struct {
	URL        string                   `json:"url"`
	RuleName   string                   `json:"rule_name"`
	RuleID     string                   `json:"rule_id"`
	Match      string                   `json:"match"`
	Source     string                   `json:"source"`
	Line       int                      `json:"line,omitempty"`
	Validation *secrets.ValidationResult `json:"validation,omitempty"`
}

// AuthInfo describes the authentication state and mechanism used during crawl.
type AuthInfo struct {
	Authenticated   bool               `json:"authenticated"`
	LoginURL        string             `json:"login_url,omitempty"`
	Username        string             `json:"username,omitempty"`
	Method          string             `json:"method,omitempty"` // "form", "token", "cookie", "header"
	SessionCookies  []SessionCookieInfo `json:"session_cookies,omitempty"`
	AuthHeaderNames []string           `json:"auth_header_names,omitempty"`
}

// SessionCookieInfo describes a session cookie (names/flags only, never values).
type SessionCookieInfo struct {
	Name     string `json:"name"`
	Domain   string `json:"domain"`
	Path     string `json:"path"`
	HTTPOnly bool   `json:"http_only"`
	Secure   bool   `json:"secure"`
	SameSite string `json:"same_site,omitempty"`
}

// GraphStats summarizes the crawl graph topology.
type GraphStats struct {
	TotalPages int `json:"total_pages"`
	MaxDepth   int `json:"max_depth"`
	TotalEdges int `json:"total_edges"`
	TotalForms int `json:"total_forms"`
}

// Collector aggregates crawl results for report generation. Thread-safe.
type Collector struct {
	mu           sync.Mutex
	results      []*output.Result
	technologies map[string]bool
	secrets      []SecretFinding
	startTime    time.Time
	targetURL    string

	// Authentication info
	authInfo *AuthInfo

	// Static analysis findings
	frameworks       []FrameworkInfo
	routes           []RouteInfo
	sourceMaps       []string
	postMessageRisks []PostMessageFinding
	graphQLOps       []GraphQLOpInfo
}

// NewCollector creates a result collector for report generation.
func NewCollector(targetURL string) *Collector {
	return &Collector{
		technologies: make(map[string]bool),
		startTime:    time.Now(),
		targetURL:    targetURL,
	}
}

// AddSecrets adds secret findings from async scanning. Thread-safe.
func (c *Collector) AddSecrets(url string, findings []secrets.Finding) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, s := range findings {
		c.secrets = append(c.secrets, SecretFinding{
			URL:        url,
			RuleName:   s.RuleName,
			RuleID:     s.RuleID,
			Match:      s.Match,
			Source:     s.Source,
			Line:       s.Line,
			Validation: s.Validation,
		})
	}
}

// SetAuthInfo sets authentication details for the report. Thread-safe.
func (c *Collector) SetAuthInfo(info AuthInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authInfo = &info
}

// AddFrameworks adds detected framework info, deduplicating by name. Thread-safe.
func (c *Collector) AddFrameworks(frameworks []FrameworkInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, fw := range frameworks {
		found := false
		for i, existing := range c.frameworks {
			if existing.Name == fw.Name {
				found = true
				// Merge evidence and upgrade version if found
				if fw.Version != "" && existing.Version == "" {
					c.frameworks[i].Version = fw.Version
				}
				// Merge unique evidence
				evidenceSet := make(map[string]bool)
				for _, e := range existing.Evidence {
					evidenceSet[e] = true
				}
				for _, e := range fw.Evidence {
					if !evidenceSet[e] {
						c.frameworks[i].Evidence = append(c.frameworks[i].Evidence, e)
					}
				}
				break
			}
		}
		if !found {
			c.frameworks = append(c.frameworks, fw)
		}
	}
}

// AddRoutes adds discovered route definitions, deduplicating by path. Thread-safe.
func (c *Collector) AddRoutes(routes []RouteInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	seen := make(map[string]bool)
	for _, existing := range c.routes {
		seen[existing.Path] = true
	}
	for _, r := range routes {
		if !seen[r.Path] {
			seen[r.Path] = true
			c.routes = append(c.routes, r)
		}
	}
}

// AddSourceMaps adds source map URLs to the report. Thread-safe.
func (c *Collector) AddSourceMaps(urls []string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.sourceMaps = append(c.sourceMaps, urls...)
}

// AddPostMessageFindings adds postMessage security findings, deduplicating
// by type+handler+target+severity. Thread-safe.
func (c *Collector) AddPostMessageFindings(findings []PostMessageFinding) {
	c.mu.Lock()
	defer c.mu.Unlock()

	seen := make(map[string]bool)
	for _, existing := range c.postMessageRisks {
		key := existing.Type + "|" + existing.HandlerType + "|" + existing.TargetOrigin + "|" + existing.Severity
		if len(existing.DataSinks) > 0 {
			key += "|" + strings.Join(existing.DataSinks, ",")
		}
		seen[key] = true
	}

	for _, pm := range findings {
		key := pm.Type + "|" + pm.HandlerType + "|" + pm.TargetOrigin + "|" + pm.Severity
		if len(pm.DataSinks) > 0 {
			key += "|" + strings.Join(pm.DataSinks, ",")
		}
		if seen[key] {
			continue
		}
		seen[key] = true
		c.postMessageRisks = append(c.postMessageRisks, pm)
	}
}

// AddGraphQLOps adds discovered GraphQL operations, deduplicating by type+name. Thread-safe.
func (c *Collector) AddGraphQLOps(ops []GraphQLOpInfo) {
	c.mu.Lock()
	defer c.mu.Unlock()
	seen := make(map[string]bool)
	for _, existing := range c.graphQLOps {
		seen[existing.Type+":"+existing.Name] = true
	}
	for _, op := range ops {
		key := op.Type + ":" + op.Name
		if !seen[key] {
			seen[key] = true
			c.graphQLOps = append(c.graphQLOps, op)
		}
	}
}

// Collect adds a crawl result to the collector. Thread-safe.
func (c *Collector) Collect(result *output.Result) {
	if result == nil || result.Request == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	c.results = append(c.results, result)

	if result.Response != nil {
		for _, tech := range result.Response.Technologies {
			c.technologies[tech] = true
		}
	}
}

// GenerateReport builds the final SiteReport from all collected results.
func (c *Collector) GenerateReport() *SiteReport {
	c.mu.Lock()
	defer c.mu.Unlock()

	r := &SiteReport{
		TargetURL:     c.targetURL,
		CrawlTime:     c.startTime,
		CrawlDuration: time.Since(c.startTime),
		TotalRequests: len(c.results),
	}

	for tech := range c.technologies {
		r.Technologies = append(r.Technologies, tech)
	}
	sort.Strings(r.Technologies)

	seen := make(map[string]bool)
	var endpoints []Endpoint
	var forms []FormInfo
	headerChecked := make(map[string]bool)
	var headerFindings []HeaderFinding

	for _, result := range c.results {
		// Deduplicate by method + URL with noise params stripped.
		// This collapses e.g. /task?_rsc=3hm2o and /task?_rsc=dtgoq into one entry.
		dedupURL := stripNoiseParams(result.Request.URL)
		key := result.Request.Method + " " + dedupURL
		if seen[key] {
			continue
		}
		seen[key] = true

		ep := buildEndpoint(result)
		endpoints = append(endpoints, ep)

		// Collect forms
		if result.Response != nil {
			for _, f := range result.Response.Forms {
				forms = append(forms, FormInfo{
					Action:     f.Action,
					Method:     f.Method,
					Parameters: f.Parameters,
					Enctype:    f.Enctype,
					PageURL:    result.Request.URL,
				})
			}
		}

		// Security header analysis (once per unique URL)
		if result.Response != nil && result.Response.StatusCode == 200 && !headerChecked[result.Request.URL] {
			headerChecked[result.Request.URL] = true
			findings := analyzeSecurityHeaders(result.Request.URL, result.Response.Headers)
			headerFindings = append(headerFindings, findings...)
		}

		// XHR-discovered endpoints
		if result.Response != nil {
			for _, xhr := range result.Response.XhrRequests {
				xhrKey := xhr.Method + " " + xhr.URL
				if seen[xhrKey] {
					continue
				}
				seen[xhrKey] = true
				xhrEp := Endpoint{
					URL:        xhr.URL,
					Method:     xhr.Method,
					Categories: classifyEndpoint(xhr.URL, xhr.Method, nil, false),
					Source:     "xhr",
				}
				if u, err := url.Parse(xhr.URL); err == nil {
					interesting := filterNoiseParams(u.Query())
					xhrEp.HasParams = len(interesting) > 0
					for k := range interesting {
						xhrEp.ParamNames = append(xhrEp.ParamNames, k)
					}
				}
				endpoints = append(endpoints, xhrEp)
			}
		}
	}

	r.Endpoints = endpoints
	r.Forms = forms
	r.SecurityHeaders = deduplicateHeaderFindings(headerFindings)
	r.Secrets = c.secrets
	r.AttackSurface = classifyAttackSurface(endpoints)
	r.AttackSurface.Summary.SecretsFound = len(c.secrets)

	// Authentication
	r.Auth = c.authInfo

	// Static analysis findings
	r.Frameworks = c.frameworks
	r.Routes = c.routes
	r.SourceMaps = c.sourceMaps
	r.PostMessageRisks = c.postMessageRisks
	r.GraphQLOps = c.graphQLOps

	return r
}

func buildEndpoint(result *output.Result) Endpoint {
	ep := Endpoint{
		URL:    result.Request.URL,
		Method: result.Request.Method,
		Source: result.Request.Source,
		Depth:  result.Request.Depth,
	}

	if result.Response != nil {
		ep.StatusCode = result.Response.StatusCode
		ep.Technologies = result.Response.Technologies
		ep.FormCount = len(result.Response.Forms)
		if ct, ok := result.Response.Headers["content-type"]; ok {
			ep.ContentType = ct
		}
	}

	// Extract query parameter names (exclude framework noise params)
	if u, err := url.Parse(result.Request.URL); err == nil {
		interesting := filterNoiseParams(u.Query())
		ep.HasParams = len(interesting) > 0
		for k := range interesting {
			ep.ParamNames = append(ep.ParamNames, k)
		}
		sort.Strings(ep.ParamNames)
	}

	// Classify
	hasForms := ep.FormCount > 0
	var respForms []navigation.Form
	if result.Response != nil {
		respForms = result.Response.Forms
	}
	ep.Categories = classifyEndpoint(result.Request.URL, result.Request.Method, respForms, hasForms)

	return ep
}

func classifyAttackSurface(endpoints []Endpoint) AttackSurface {
	as := AttackSurface{
		Summary: AttackSurfaceSummary{
			TotalEndpoints: len(endpoints),
			ByCategory:     make(map[string]int),
			ByMethod:       make(map[string]int),
			ByStatusCode:   make(map[int]int),
		},
	}

	hosts := make(map[string]bool)

	for _, ep := range endpoints {
		as.Summary.ByMethod[ep.Method]++
		if ep.StatusCode > 0 {
			as.Summary.ByStatusCode[ep.StatusCode]++
		}

		if u, err := url.Parse(ep.URL); err == nil {
			hosts[u.Hostname()] = true
		}

		if len(ep.Categories) == 0 {
			as.Uncategorized = append(as.Uncategorized, ep)
			as.Summary.Uncategorized++
			continue
		}

		for _, cat := range ep.Categories {
			as.Summary.ByCategory[cat]++

			switch cat {
			case "auth":
				as.AuthEndpoints = append(as.AuthEndpoints, ep)
			case "admin":
				as.AdminEndpoints = append(as.AdminEndpoints, ep)
			case "file":
				as.FileEndpoints = append(as.FileEndpoints, ep)
			case "search":
				as.SearchEndpoints = append(as.SearchEndpoints, ep)
			case "mutation":
				as.DataMutation = append(as.DataMutation, ep)
			case "user_specific":
				as.UserSpecific = append(as.UserSpecific, ep)
			case "api":
				as.APIEndpoints = append(as.APIEndpoints, ep)
			case "external_input":
				as.ExternalInputs = append(as.ExternalInputs, ep)
			case "parameterized":
				as.Parameterized = append(as.Parameterized, ep)
				as.Summary.WithParams++
			case "has_form":
				as.HasForms = append(as.HasForms, ep)
				as.Summary.WithForms++
			}
		}
	}

	as.Summary.UniqueHosts = len(hosts)
	return as
}

// classifyEndpoint assigns security categories based on URL patterns, method, forms, and parameters.
func classifyEndpoint(rawURL, method string, forms []navigation.Form, hasForms bool) []string {
	var categories []string
	lowerURL := strings.ToLower(rawURL)

	path := ""
	var queryParams url.Values
	if u, err := url.Parse(rawURL); err == nil {
		path = strings.ToLower(u.Path)
		queryParams = u.Query()
	}

	// Static assets (JS chunks, CSS, fonts, images) are noise — skip most classifications
	isStatic := staticAssetPattern.MatchString(rawURL)

	// URL path-based classification
	if authPattern.MatchString(path) {
		categories = append(categories, "auth")
	}
	if adminPattern.MatchString(path) {
		categories = append(categories, "admin")
	}
	if !isStatic && filePattern.MatchString(path) {
		categories = append(categories, "file")
	}
	for _, f := range forms {
		if strings.Contains(f.Enctype, "multipart") {
			categories = append(categories, "file")
			break
		}
	}
	if searchPattern.MatchString(path) || searchParamPattern.MatchString(lowerURL) {
		categories = append(categories, "search")
	}
	if method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE" {
		categories = append(categories, "mutation")
	}
	if idorPattern.MatchString(path) {
		categories = append(categories, "user_specific")
	}
	if apiPattern.MatchString(path) {
		categories = append(categories, "api")
	}
	if ssrfParamPattern.MatchString(lowerURL) {
		categories = append(categories, "external_input")
	}

	// Parameterized endpoints — only count if there are interesting (non-framework) query params
	interestingParams := filterNoiseParams(queryParams)
	if len(interestingParams) > 0 {
		categories = append(categories, "parameterized")
	}

	// Pages with forms — forms are the primary interaction surface
	if hasForms {
		categories = append(categories, "has_form")
	}

	// PHP/dynamic file extensions are interesting
	if dynamicExtPattern.MatchString(path) {
		// Don't add a category, but this is noted — these get classified by other signals
	}

	return uniqueStrings(categories)
}

// Classification patterns
var (
	authPattern = regexp.MustCompile(`(?i)/(login|signin|sign-in|signup|sign-up|register|auth|oauth|sso|cas|logout|signout|sign-out|password|reset-password|forgot|recover|verify|confirm|activate|token|session|2fa|mfa|otp|brute)(/|$|\.)`)

	adminPattern = regexp.MustCompile(`(?i)/(admin|administrator|manage|management|control|panel|console|dashboard|backend|cms|wp-admin|staff|internal|system|config|configuration|settings|setup|security|phpinfo|server-status|server-info)(/|$|\.)`)

	filePattern = regexp.MustCompile(`(?i)/(upload|download|file|files|attachment|attachments|document|documents|import|export|backup|restore|assets/upload)(/|$|\.)`)

	// staticAssetPattern matches paths that are build-time static assets (JS chunks, CSS, fonts, images).
	// These are noise in attack surface reports — not interesting injection or interaction targets.
	staticAssetPattern = regexp.MustCompile(`(?i)(_next/static/|/static/(chunks|css|media|js)/|/dist/|/vendor/|/node_modules/|\.(woff2?|ttf|eot|ico|png|jpg|jpeg|gif|svg|webp|map)($|\?))`)

	searchPattern = regexp.MustCompile(`(?i)/(search|find|query|filter|lookup|browse|explore|autocomplete|suggest|typeahead|exec|command|sqli|injection|xss)(/|$|\.)`)

	searchParamPattern = regexp.MustCompile(`(?i)[?&](q|query|search|keyword|term|filter|find|s|cmd|exec|command|id|name|ip)=`)

	idorPattern = regexp.MustCompile(`(?i)/(users?|accounts?|profiles?|orders?|invoices?|messages?|tickets?|documents?|reports?)/(\d+|[a-f0-9-]{36}|\{[^}]+\})(/|$)`)

	apiPattern = regexp.MustCompile(`(?i)/(api|graphql|rest|v[0-9]+|ws|websocket|webhook|callback|rpc|grpc|json|xml)(/|$)`)

	ssrfParamPattern = regexp.MustCompile(`(?i)[?&](url|uri|link|redirect|next|return|callback|target|dest|destination|site|page|go|out|view|ref|feed|host|domain|proxy|path|file|include|require|src|source)=`)

	dynamicExtPattern = regexp.MustCompile(`(?i)\.(php|asp|aspx|jsp|cgi|pl|py|rb|action|do)($|\?)`)
)

// Security headers we check for
var securityHeaders = []struct {
	name     string
	required bool // whether absence is a finding
}{
	{"x-frame-options", true},
	{"x-content-type-options", true},
	{"content-security-policy", true},
	{"strict-transport-security", true},
	{"x-xss-protection", false}, // deprecated but still interesting
	{"access-control-allow-origin", false}, // CORS — presence is interesting
}

func analyzeSecurityHeaders(pageURL string, headers navigation.Headers) []HeaderFinding {
	if headers == nil {
		return nil
	}

	var findings []HeaderFinding
	lowerHeaders := make(map[string]string)
	for k, v := range headers {
		lowerHeaders[strings.ToLower(k)] = v
	}

	for _, sh := range securityHeaders {
		val, present := lowerHeaders[sh.name]
		if !present && sh.required {
			findings = append(findings, HeaderFinding{
				URL:    pageURL,
				Header: sh.name,
				Status: "missing",
			})
		} else if present {
			if sh.name == "access-control-allow-origin" && val == "*" {
				findings = append(findings, HeaderFinding{
					URL:    pageURL,
					Header: sh.name,
					Status: "weak",
					Value:  val,
				})
			}
		}
	}
	return findings
}

func deduplicateHeaderFindings(findings []HeaderFinding) []HeaderFinding {
	// Deduplicate by header name (report unique missing headers, not per-URL)
	seen := make(map[string]bool)
	var result []HeaderFinding
	for _, f := range findings {
		key := f.Header + "|" + f.Status
		if !seen[key] {
			seen[key] = true
			result = append(result, f)
		}
	}
	return result
}

// noiseParams are framework-internal query parameters that don't represent
// user-controllable injection surface. They're stripped before classifying
// an endpoint as "parameterized".
var noiseParams = map[string]bool{
	// Next.js / React Server Components
	"_rsc": true, "dpl": true, "__N": true,
	// Clerk auth SDK
	"__clerk_api_version": true, "_clerk_js_version": true,
	// Vercel / deployment
	"__vercel_draft_mode": true,
	// Cache busters (generic)
	"_": true, "t": true, "ts": true, "cb": true, "nocache": true,
}

// filterNoiseParams returns query params that are NOT known framework noise.
func filterNoiseParams(params url.Values) url.Values {
	interesting := make(url.Values)
	for k, v := range params {
		if !noiseParams[k] {
			interesting[k] = v
		}
	}
	return interesting
}

// stripNoiseParams removes known framework query params from a URL for deduplication.
// Returns the URL with only interesting params, so /task?_rsc=abc → /task
func stripNoiseParams(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	interesting := filterNoiseParams(u.Query())
	if len(interesting) == 0 {
		u.RawQuery = ""
	} else {
		u.RawQuery = interesting.Encode()
	}
	return u.String()
}

// stripParamValues replaces query param values with placeholders for dedup grouping.
// e.g., /sign-in?redirectUrl=https%3A//foo → /sign-in?redirectUrl={value}
func stripParamValues(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	params := filterNoiseParams(u.Query())
	if len(params) == 0 {
		u.RawQuery = ""
		return u.String()
	}
	normalized := make(url.Values)
	for k := range params {
		normalized.Set(k, "{value}")
	}
	u.RawQuery = normalized.Encode()
	return u.String()
}

// IsStaticAsset returns true if the URL path matches known static asset patterns
// (JS build chunks, CSS, fonts, images) that are noise in attack surface reports.
func IsStaticAsset(rawURL string) bool {
	return staticAssetPattern.MatchString(rawURL)
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]bool, len(s))
	result := make([]string, 0, len(s))
	for _, v := range s {
		if !seen[v] {
			seen[v] = true
			result = append(result, v)
		}
	}
	return result
}
