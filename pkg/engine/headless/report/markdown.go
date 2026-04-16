package report

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/projectdiscovery/katana/pkg/engine/headless/graph"
)

// uuidPattern matches UUIDs in URL paths for collapsing duplicate templated routes.
var uuidPattern = regexp.MustCompile(`[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}`)

// sessionTokenPattern matches session/request-specific tokens like Clerk's sess_xxx, sia_xxx, etc.
var sessionTokenPattern = regexp.MustCompile(`(sess|sia|sin|srt|tok|txn)_[A-Za-z0-9]{20,}`)

// replaceTemplatedSegments replaces UUIDs and session tokens with template placeholders.
func replaceTemplatedSegments(s string) string {
	s = uuidPattern.ReplaceAllString(s, "{uuid}")
	s = sessionTokenPattern.ReplaceAllString(s, "{token}")
	return s
}

// containsTemplatedSegment returns true if the string contains a UUID or session token.
func containsTemplatedSegment(s string) bool {
	return uuidPattern.MatchString(s) || sessionTokenPattern.MatchString(s)
}

// displayURL strips noise params and replaces session tokens for cleaner display.
func displayURL(rawURL string) string {
	cleaned := stripNoiseParams(rawURL)
	return replaceTemplatedSegments(cleaned)
}

// RenderMarkdown writes a detailed human-readable attack surface report.
func RenderMarkdown(w io.Writer, r *SiteReport) error {
	p := func(format string, args ...any) {
		fmt.Fprintf(w, format+"\n", args...)
	}

	// Header
	p("# Attack Surface Report")
	p("")
	p("**Target:** %s", r.TargetURL)
	p("**Crawl Time:** %s", r.CrawlTime.Format("2006-01-02 15:04:05"))
	p("**Duration:** %s", r.CrawlDuration.Round(1e9))
	p("**Total Requests:** %d", r.TotalRequests)
	p("**Unique Endpoints:** %d", len(r.Endpoints))
	if r.GraphStats != nil {
		p("**Unique Pages:** %d (max depth: %d)", r.GraphStats.TotalPages, r.GraphStats.MaxDepth)
	}
	p("")

	// Authentication
	if r.Auth != nil {
		p("## Authentication")
		p("")
		if r.Auth.Authenticated {
			p("**Status:** Authenticated")
			p("**Login URL:** %s", r.Auth.LoginURL)
			p("**Username:** %s", r.Auth.Username)
			p("**Method:** %s", r.Auth.Method)
			p("")
			if len(r.Auth.SessionCookies) > 0 {
				p("### Session Cookies")
				p("")
				p("| Name | Domain | Path | HttpOnly | Secure | SameSite |")
				p("|------|--------|------|:--------:|:------:|----------|")
				for _, c := range r.Auth.SessionCookies {
					httpOnly := "No"
					if c.HTTPOnly {
						httpOnly = "Yes"
					}
					secure := "No"
					if c.Secure {
						secure = "Yes"
					}
					sameSite := c.SameSite
					if sameSite == "" {
						sameSite = "-"
					}
					p("| `%s` | %s | %s | %s | %s | %s |", c.Name, c.Domain, c.Path, httpOnly, secure, sameSite)
				}
				p("")
			}
			if len(r.Auth.AuthHeaderNames) > 0 {
				p("### Auth Headers")
				p("")
				for _, h := range r.Auth.AuthHeaderNames {
					p("- `%s`", h)
				}
				p("")
			}

			// Show auth boundary — which endpoint categories are behind auth
			p("### Auth-Protected Surface")
			p("")
			p("> All endpoints below were discovered while authenticated. Pages only visible to `%s`:", r.Auth.Username)
			p("")
			as := r.AttackSurface
			protectedCount := 0
			if len(as.AdminEndpoints) > 0 {
				p("- **Admin endpoints:** %d", len(as.AdminEndpoints))
				protectedCount += len(as.AdminEndpoints)
			}
			if len(as.UserSpecific) > 0 {
				p("- **User-specific endpoints (IDOR candidates):** %d", len(as.UserSpecific))
				protectedCount += len(as.UserSpecific)
			}
			if len(as.DataMutation) > 0 {
				p("- **State-changing endpoints:** %d", len(as.DataMutation))
				protectedCount += len(as.DataMutation)
			}
			if len(as.APIEndpoints) > 0 {
				p("- **API endpoints:** %d", len(as.APIEndpoints))
				protectedCount += len(as.APIEndpoints)
			}
			totalEps := len(r.Endpoints)
			if totalEps > 0 && protectedCount > 0 {
				p("")
				p("**%d of %d endpoints** (%.0f%%) were discovered under this authenticated session.",
					protectedCount, totalEps, float64(protectedCount)/float64(totalEps)*100)
			}
			p("")
		} else {
			p("**Status:** Unauthenticated")
			p("")
			p("> Crawl ran without authentication. Auth-gated content was not explored.")
			p("")
		}
	}

	// Technologies
	if len(r.Technologies) > 0 {
		p("## Technologies Detected")
		p("")
		for _, tech := range r.Technologies {
			p("- %s", tech)
		}
		p("")
	}

	// Summary table
	as := r.AttackSurface
	p("## Attack Surface Summary")
	p("")
	p("| Category | Count | Description |")
	p("|----------|------:|-------------|")
	summaryRow(w, "Auth", as.Summary.ByCategory["auth"], "Login, register, password reset, OAuth, session endpoints")
	summaryRow(w, "Admin", as.Summary.ByCategory["admin"], "Admin panels, management, dashboards, config pages")
	summaryRow(w, "File Operations", as.Summary.ByCategory["file"], "Upload, download, import/export, attachments")
	summaryRow(w, "Search / Injection", as.Summary.ByCategory["search"], "Search, query, command endpoints (injection candidates)")
	summaryRow(w, "Data Mutation", as.Summary.ByCategory["mutation"], "POST/PUT/PATCH/DELETE endpoints (state-changing)")
	summaryRow(w, "User-Specific", as.Summary.ByCategory["user_specific"], "User/account/order endpoints (IDOR candidates)")
	summaryRow(w, "API", as.Summary.ByCategory["api"], "REST, GraphQL, WebSocket, webhook endpoints")
	summaryRow(w, "External Inputs", as.Summary.ByCategory["external_input"], "URL/redirect/include parameters (SSRF/LFI candidates)")
	summaryRow(w, "Parameterized", as.Summary.WithParams, "Endpoints with query parameters (injection surface)")
	summaryRow(w, "Pages with Forms", as.Summary.WithForms, "Pages containing HTML forms (interaction surface)")
	summaryRow(w, "Leaked Secrets", as.Summary.SecretsFound, "Secrets detected in HTTP traffic (API keys, tokens, credentials)")
	p("")

	// Methods + Status codes
	p("**Methods:** %s", formatCounts(as.Summary.ByMethod))
	if len(as.Summary.ByStatusCode) > 0 {
		p("**Status Codes:** %s", formatIntCounts(as.Summary.ByStatusCode))
	}
	p("")

	// Discovered Hosts — group all endpoints by hostname with request counts
	renderDiscoveredHosts(w, r)

	// Category detail sections
	renderCategoryDetail(w, "Authentication Endpoints", as.AuthEndpoints,
		"Targets for: credential stuffing, auth bypass, session fixation, 2FA bypass, brute force")
	renderCategoryDetail(w, "Admin Endpoints", as.AdminEndpoints,
		"Targets for: privilege escalation, auth bypass, default credentials, information disclosure")
	renderCategoryDetail(w, "File Operation Endpoints", as.FileEndpoints,
		"Targets for: unrestricted upload, path traversal, RCE via file upload, XXE")
	renderCategoryDetail(w, "Search / Injection Endpoints", as.SearchEndpoints,
		"Targets for: SQL injection, command injection, XSS, SSTI, LDAP injection")
	renderCategoryDetail(w, "Data Mutation Endpoints", as.DataMutation,
		"Targets for: CSRF, mass assignment, broken access control, race conditions")
	renderCategoryDetail(w, "User-Specific Endpoints (IDOR Candidates)", as.UserSpecific,
		"Targets for: IDOR, horizontal privilege escalation, data leakage")
	renderCategoryDetail(w, "API Endpoints", as.APIEndpoints,
		"Targets for: broken auth, excessive data exposure, rate limiting, injection")
	renderCategoryDetail(w, "External Input Endpoints (SSRF/LFI Candidates)", as.ExternalInputs,
		"Targets for: SSRF, LFI/RFI, open redirect, header injection")

	// Parameterized endpoints — important injection surface (static assets filtered)
	if len(as.Parameterized) > 0 {
		var interestingParam []Endpoint
		var staticParamCount int
		for _, ep := range as.Parameterized {
			if IsStaticAsset(ep.URL) {
				staticParamCount++
			} else {
				interestingParam = append(interestingParam, ep)
			}
		}

		if len(interestingParam) > 0 {
			p("## Parameterized Endpoints")
			p("")
			p("> Every query parameter is a potential injection point")
			if staticParamCount > 0 {
				p("> (%d static asset URLs with framework params omitted)", staticParamCount)
			}
			p("")
			p("| Method | URL | Parameters | Status |")
			p("|--------|-----|------------|--------|")
			for _, ep := range interestingParam {
				params := strings.Join(ep.ParamNames, ", ")
				status := fmtStatus(ep.StatusCode)
				p("| %s | %s | `%s` | %s |", ep.Method, trunc(displayURL(ep.URL), 60), params, status)
			}
			p("")
		}
	}

	// Pages with forms
	if len(as.HasForms) > 0 {
		p("## Pages with Forms")
		p("")
		p("> Forms are the primary user interaction surface")
		p("")
		p("| URL | Forms | Status |")
		p("|-----|------:|--------|")
		for _, ep := range as.HasForms {
			p("| %s | %d | %s |", trunc(ep.URL, 70), ep.FormCount, fmtStatus(ep.StatusCode))
		}
		p("")
	}

	// Forms detail
	if len(r.Forms) > 0 {
		p("## Discovered Forms (Detail)")
		p("")
		p("| Page | Action | Method | Fields |")
		p("|------|--------|--------|--------|")
		for _, f := range r.Forms {
			params := strings.Join(f.Parameters, ", ")
			if len(params) > 50 {
				params = params[:47] + "..."
			}
			if params == "" {
				params = "-"
			}
			p("| %s | %s | %s | %s |", trunc(f.PageURL, 35), trunc(f.Action, 30), f.Method, params)
		}
		p("")
	}

	// Security headers
	if len(r.SecurityHeaders) > 0 {
		p("## Security Header Analysis")
		p("")
		p("| Header | Status | Details |")
		p("|--------|--------|---------|")
		for _, f := range r.SecurityHeaders {
			detail := ""
			if f.Value != "" {
				detail = "`" + f.Value + "`"
			}
			status := f.Status
			if status == "missing" {
				status = "**MISSING**"
			} else if status == "weak" {
				status = "**WEAK**"
			}
			p("| `%s` | %s | %s |", f.Header, status, detail)
		}
		p("")
	}

	// Leaked secrets — grouped by rule to reduce noise
	if len(r.Secrets) > 0 {
		p("## Leaked Secrets Detected")
		p("")
		p("> Secrets found in HTTP traffic during crawl (%d total findings)", len(r.Secrets))
		p("")

		// Group by rule name
		type secretGroup struct {
			rule       string
			first      SecretFinding
			urls       []string
			count      int
			hasActive  bool
		}
		groupOrder := []string{}
		groups := map[string]*secretGroup{}
		for _, s := range r.Secrets {
			g, ok := groups[s.RuleName]
			if !ok {
				g = &secretGroup{rule: s.RuleName, first: s}
				groups[s.RuleName] = g
				groupOrder = append(groupOrder, s.RuleName)
			}
			g.count++
			// Track unique URLs (up to 5 for display)
			if len(g.urls) < 5 {
				dup := false
				for _, u := range g.urls {
					if u == s.URL {
						dup = true
						break
					}
				}
				if !dup {
					g.urls = append(g.urls, s.URL)
				}
			}
			if s.Validation != nil && s.Validation.Status == "valid" {
				g.hasActive = true
			}
		}

		p("| Rule | Sample Match | Occurrences | Unique URLs | Validation |")
		p("|------|-------------|------------:|------------:|------------|")
		for _, name := range groupOrder {
			g := groups[name]
			validation := "-"
			if g.hasActive {
				validation = "**ACTIVE**"
			} else if g.first.Validation != nil {
				validation = string(g.first.Validation.Status)
			}
			match := trunc(g.first.Match, 40)
			p("| %s | `%s` | %d | %d | %s |", g.rule, match, g.count, len(g.urls), validation)
		}
		p("")

		// Show URLs per group
		for _, name := range groupOrder {
			g := groups[name]
			p("**%s** (%d occurrences)", g.rule, g.count)
			p("")
			for _, u := range g.urls {
				p("- %s", trunc(u, 80))
			}
			if g.count > len(g.urls) {
				p("- ... and %d more", g.count-len(g.urls))
			}
			p("")
		}
	}

	// Frameworks detected via jsluice static analysis
	if len(r.Frameworks) > 0 {
		p("## Detected Frameworks")
		p("")
		p("| Framework | Version | Evidence |")
		p("|-----------|---------|----------|")
		for _, fw := range r.Frameworks {
			version := fw.Version
			if version == "" {
				version = "-"
			}
			evidence := strings.Join(fw.Evidence, ", ")
			if len(evidence) > 60 {
				evidence = evidence[:57] + "..."
			}
			p("| **%s** | %s | %s |", fw.Name, version, evidence)
		}
		p("")
	}

	// Route definitions from JS bundles
	if len(r.Routes) > 0 {
		p("## Discovered Routes (from JS bundles)")
		p("")
		p("> Route definitions extracted from framework router configs — reveals app surface without visiting pages")
		p("")
		p("| Path | Method | Framework | Parameters |")
		p("|------|--------|-----------|------------|")
		for _, route := range r.Routes {
			method := route.Method
			if method == "" {
				method = "-"
			}
			params := strings.Join(route.ParamNames, ", ")
			if params == "" {
				params = "-"
			}
			p("| `%s` | %s | %s | %s |", route.Path, method, route.Framework, params)
		}
		p("")
	}

	// Source maps (security finding)
	if len(r.SourceMaps) > 0 {
		p("## Source Maps Detected (Information Disclosure)")
		p("")
		p("> **FINDING:** Exposed source maps reveal unminified source code. Severity: Medium")
		p("")
		for _, sm := range r.SourceMaps {
			p("- %s", sm)
		}
		p("")
	}

	// postMessage security findings
	if len(r.PostMessageRisks) > 0 {
		// Split into listeners and senders for cleaner reporting
		var listeners, senders []PostMessageFinding
		for _, pm := range r.PostMessageRisks {
			if pm.Type == "listener" {
				listeners = append(listeners, pm)
			} else {
				senders = append(senders, pm)
			}
		}

		p("## postMessage Security Findings")
		p("")

		if len(listeners) > 0 {
			p("### Message Listeners")
			p("")
			p("> Handlers without origin validation accept messages from any origin — XSS/data exfiltration risk")
			p("")
			p("| Origin Check | Sinks | Severity | Source |")
			p("|:------------:|-------|----------|--------|")
			for _, pm := range listeners {
				originCheck := "**No**"
				if pm.HasOriginCheck {
					origins := strings.Join(pm.AllowedOrigins, ", ")
					if origins != "" {
						originCheck = origins
					} else {
						originCheck = "Yes"
					}
				}
				sinks := strings.Join(pm.DataSinks, ", ")
				if sinks == "" {
					sinks = "-"
				}
				sev := pm.Severity
				if sev == "high" {
					sev = "**HIGH**"
				} else if sev == "medium" {
					sev = "**MEDIUM**"
				}
				source := pm.Filename
				if source == "" {
					source = "-"
				}
				p("| %s | %s | %s | %s |", originCheck, sinks, sev, trunc(source, 50))
			}
			p("")
		}

		if len(senders) > 0 {
			p("### Message Senders")
			p("")
			p("> `postMessage()` calls — wildcard `*` target leaks data to any embedder")
			p("")
			p("| Target Origin | Severity | Source |")
			p("|---------------|----------|--------|")
			for _, pm := range senders {
				target := pm.TargetOrigin
				if target == "" {
					target = "EXPR"
				}
				if target == "*" {
					target = "**`*` (wildcard)**"
				} else {
					target = "`" + target + "`"
				}
				source := pm.Filename
				if source == "" {
					source = "-"
				}
				p("| %s | %s | %s |", target, pm.Severity, trunc(source, 50))
			}
			p("")
		}
	}

	// GraphQL operations
	if len(r.GraphQLOps) > 0 {
		p("## GraphQL Operations")
		p("")
		p("| Type | Name |")
		p("|------|------|")
		for _, gql := range r.GraphQLOps {
			p("| %s | %s |", gql.Type, gql.Name)
		}
		p("")
	}

	// All endpoints (complete inventory) — filter out static assets and OPTIONS for readability
	var appEndpoints, staticEndpoints []Endpoint
	optionsCount := 0
	for _, ep := range r.Endpoints {
		if ep.Method == "OPTIONS" {
			optionsCount++
			continue
		}
		if IsStaticAsset(ep.URL) {
			staticEndpoints = append(staticEndpoints, ep)
		} else {
			appEndpoints = append(appEndpoints, ep)
		}
	}

	// Collapse endpoints with UUIDs in the path into templated groups
	type inventoryRow struct {
		ep       Endpoint
		template string // URL with UUIDs replaced, or empty if not templated
		count    int
	}
	var rows []inventoryRow
	templateGroups := make(map[string]*inventoryRow) // key: method + templated URL
	var templateOrder []string

	for _, ep := range appEndpoints {
		if containsTemplatedSegment(ep.URL) {
			tmpl := displayURL(ep.URL)
			key := ep.Method + " " + tmpl
			if g, ok := templateGroups[key]; ok {
				g.count++
			} else {
				row := &inventoryRow{ep: ep, template: tmpl, count: 1}
				templateGroups[key] = row
				templateOrder = append(templateOrder, key)
			}
		} else {
			// Collapse endpoints that share the same path but differ only in param values
			// e.g., /sign-in?redirectUrl=A and /sign-in?redirectUrl=B → one row
			stripped := stripParamValues(ep.URL)
			key := ep.Method + " " + stripped
			if len(ep.ParamNames) > 0 {
				if g, ok := templateGroups[key]; ok {
					g.count++
				} else {
					row := &inventoryRow{ep: ep, template: displayURL(ep.URL), count: 1}
					templateGroups[key] = row
					templateOrder = append(templateOrder, key)
				}
			} else {
				rows = append(rows, inventoryRow{ep: ep, count: 1})
			}
		}
	}
	// Append templated groups in discovery order
	for _, key := range templateOrder {
		rows = append(rows, *templateGroups[key])
	}

	p("## Complete Endpoint Inventory")
	p("")
	omitted := []string{}
	if len(staticEndpoints) > 0 {
		omitted = append(omitted, fmt.Sprintf("%d static assets (JS chunks, CSS, fonts)", len(staticEndpoints)))
	}
	if optionsCount > 0 {
		omitted = append(omitted, fmt.Sprintf("%d OPTIONS preflight requests", optionsCount))
	}
	if len(omitted) > 0 {
		p("> %s omitted — showing %d application endpoints",
			strings.Join(omitted, ", "), len(rows))
		p("")
	}
	p("| # | Method | URL | Status | Categories | Params | Source |")
	p("|--:|--------|-----|--------|------------|--------|--------|")
	for i, row := range rows {
		ep := row.ep
		rowDisplayURL := displayURL(ep.URL)
		if row.template != "" {
			rowDisplayURL = row.template
			if row.count > 1 {
				rowDisplayURL = fmt.Sprintf("%s (%d instances)", row.template, row.count)
			}
		}
		cats := strings.Join(ep.Categories, ", ")
		if cats == "" {
			cats = "-"
		}
		params := ""
		if len(ep.ParamNames) > 0 {
			params = strings.Join(ep.ParamNames, ", ")
		}
		source := ep.Source
		if source == "" {
			source = "-"
		}
		p("| %d | %s | %s | %s | %s | %s | %s |",
			i+1, ep.Method, trunc(rowDisplayURL, 70), fmtStatus(ep.StatusCode), cats, params, source)
	}
	p("")

	// JavaScript files section — compact list of all JS bundles discovered
	var jsFiles []Endpoint
	for _, ep := range r.Endpoints {
		if ep.Method == "GET" && isJSURL(ep.URL) {
			jsFiles = append(jsFiles, ep)
		}
	}
	if len(jsFiles) > 0 {
		// Group by host
		hostJS := make(map[string][]string)
		var hostOrder []string
		for _, ep := range jsFiles {
			if u, err := url.Parse(ep.URL); err == nil {
				host := u.Host
				if _, seen := hostJS[host]; !seen {
					hostOrder = append(hostOrder, host)
				}
				hostJS[host] = append(hostJS[host], u.Path)
			}
		}
		p("## JavaScript Files (%d)", len(jsFiles))
		p("")
		for _, host := range hostOrder {
			paths := hostJS[host]
			p("**%s** (%d files)", host, len(paths))
			p("")
			for _, path := range paths {
				p("- `%s`", path)
			}
			p("")
		}
	}

	// Navigation graph visualization
	if len(r.GraphEdges) > 0 {
		p("## Navigation Graph")
		p("")
		p("```mermaid")
		renderMermaidGraph(w, r.GraphNodes, r.GraphEdges)
		p("```")
		p("")
	}

	// Site tree (hierarchical path view — target host only)
	if len(r.Endpoints) > 0 {
		p("## Site Tree")
		p("")
		p("```")
		renderSiteTree(w, r.Endpoints, r.TargetURL)
		p("```")
		p("")
	}

	// Graph stats
	if r.GraphStats != nil {
		gs := r.GraphStats
		p("## Crawl Statistics")
		p("")
		p("- **Unique pages:** %d", gs.TotalPages)
		p("- **Max depth reached:** %d", gs.MaxDepth)
		p("- **Navigation edges:** %d", gs.TotalEdges)
		p("- **Forms discovered:** %d", gs.TotalForms)
		p("- **Technologies:** %d", len(r.Technologies))
		p("")
	}

	p("---")
	p("*Generated by katana headless crawler*")

	return nil
}

// renderDiscoveredHosts outputs a table of all unique hosts found during the crawl.
func renderDiscoveredHosts(w io.Writer, r *SiteReport) {
	p := func(format string, args ...any) {
		fmt.Fprintf(w, format+"\n", args...)
	}

	// Parse target host for annotation
	var targetHost string
	if u, err := url.Parse(r.TargetURL); err == nil {
		targetHost = u.Hostname()
	}

	type hostInfo struct {
		host      string
		requests  int
		endpoints int // unique paths
	}

	// Collect per-host: request count, unique paths, and methods per path
	type pathDetail struct {
		methods map[string]bool
		count   int
	}
	hostRequests := make(map[string]int)
	hostPaths := make(map[string]map[string]*pathDetail) // host → template path → detail

	for _, ep := range r.Endpoints {
		u, err := url.Parse(ep.URL)
		if err != nil {
			continue
		}
		host := u.Hostname()
		if host == "" {
			continue
		}
		hostRequests[host]++
		if hostPaths[host] == nil {
			hostPaths[host] = make(map[string]*pathDetail)
		}
		// Collapse UUIDs and session tokens for dedup
		path := replaceTemplatedSegments(u.Path)
		if path == "" {
			path = "/"
		}
		pd := hostPaths[host][path]
		if pd == nil {
			pd = &pathDetail{methods: make(map[string]bool)}
			hostPaths[host][path] = pd
		}
		pd.methods[ep.Method] = true
		pd.count++
	}

	if len(hostRequests) <= 1 {
		return // Only one host, not interesting enough to show
	}

	// Sort hosts by request count descending
	type kv struct {
		host string
		reqs int
	}
	var sorted []kv
	for h, c := range hostRequests {
		sorted = append(sorted, kv{h, c})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].reqs > sorted[j].reqs })

	p("## Discovered Hosts")
	p("")
	p("| Host | Requests | Endpoints | Notes |")
	p("|------|----------|-----------|-------|")
	for _, s := range sorted {
		endpoints := len(hostPaths[s.host])
		note := classifyHost(s.host, targetHost)
		p("| %s | %d | %d | %s |", s.host, s.reqs, endpoints, note)
	}
	p("")

	// Per-host endpoint breakdown (skip target host — its endpoints are in the main sections)
	for _, s := range sorted {
		if s.host == targetHost {
			continue
		}
		paths := hostPaths[s.host]
		if len(paths) == 0 {
			continue
		}

		// Sort paths alphabetically
		pathList := make([]string, 0, len(paths))
		for path := range paths {
			pathList = append(pathList, path)
		}
		sort.Strings(pathList)

		note := classifyHost(s.host, targetHost)
		if note != "" {
			note = " (" + note + ")"
		}
		p("### %s%s", s.host, note)
		p("")
		p("| Method(s) | Path | Requests |")
		p("|-----------|------|----------|")
		for _, path := range pathList {
			pd := paths[path]
			methods := make([]string, 0, len(pd.methods))
			for m := range pd.methods {
				methods = append(methods, m)
			}
			sort.Strings(methods)
			p("| %s | `%s` | %d |", strings.Join(methods, ", "), path, pd.count)
		}
		p("")
	}
}

// classifyHost returns a note for the discovered hosts table.
func classifyHost(host, targetHost string) string {
	lh := strings.ToLower(host)

	if host == targetHost {
		return "**Target**"
	}

	// API backends
	if strings.HasPrefix(lh, "api.") || strings.Contains(lh, ".api.") {
		return "API backend"
	}

	// Auth providers
	for _, kw := range []string{"clerk", "auth0", "auth.", "okta", "cognito", "sso."} {
		if strings.Contains(lh, kw) {
			return "Auth provider"
		}
	}

	// CDNs
	for _, cdn := range []string{"jsdelivr.net", "cloudflare", "cdn.", "unpkg.com", "cdnjs.", "fastly", "akamai"} {
		if strings.Contains(lh, cdn) {
			return "CDN"
		}
	}

	// Analytics / tracking
	for _, t := range []string{"analytics", "segment", "mixpanel", "hotjar", "sentry", "datadog"} {
		if strings.Contains(lh, t) {
			return "Analytics"
		}
	}

	return ""
}

func summaryRow(w io.Writer, name string, count int, desc string) {
	if count > 0 {
		fmt.Fprintf(w, "| **%s** | **%d** | %s |\n", name, count, desc)
	} else {
		fmt.Fprintf(w, "| %s | %d | %s |\n", name, count, desc)
	}
}

func renderCategoryDetail(w io.Writer, title string, endpoints []Endpoint, description string) {
	// Filter out static assets from category details
	var filtered []Endpoint
	for _, ep := range endpoints {
		if !IsStaticAsset(ep.URL) {
			filtered = append(filtered, ep)
		}
	}
	if len(filtered) == 0 {
		return
	}

	fmt.Fprintf(w, "## %s\n\n", title)
	fmt.Fprintf(w, "> %s\n\n", description)
	fmt.Fprintf(w, "| Method | URL | Status | Params | Source |\n")
	fmt.Fprintf(w, "|--------|-----|--------|--------|--------|\n")

	for _, ep := range filtered {
		params := ""
		if len(ep.ParamNames) > 0 {
			params = strings.Join(ep.ParamNames, ", ")
		}
		source := ep.Source
		if source == "" {
			source = "-"
		}
		fmt.Fprintf(w, "| %s | %s | %s | %s | %s |\n",
			ep.Method, trunc(displayURL(ep.URL), 60), fmtStatus(ep.StatusCode), params, source)
	}
	fmt.Fprintln(w)
}

func formatCounts(m map[string]int) string {
	type kv struct{ k string; v int }
	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	parts := make([]string, 0, len(sorted))
	for _, s := range sorted {
		parts = append(parts, fmt.Sprintf("%s (%d)", s.k, s.v))
	}
	return strings.Join(parts, ", ")
}

func formatIntCounts(m map[int]int) string {
	type kv struct{ k, v int }
	var sorted []kv
	for k, v := range m {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].v > sorted[j].v })
	parts := make([]string, 0, len(sorted))
	for _, s := range sorted {
		parts = append(parts, fmt.Sprintf("%d (%d)", s.k, s.v))
	}
	return strings.Join(parts, ", ")
}

func fmtStatus(code int) string {
	if code > 0 {
		return fmt.Sprintf("%d", code)
	}
	return "-"
}

func trunc(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max-3] + "..."
}

// renderMermaidGraph outputs a Mermaid flowchart showing the navigation graph.
// Nodes are labeled with URL paths (not full URLs) for readability.
// Limited to 50 edges to prevent huge unreadable diagrams.
func renderMermaidGraph(w io.Writer, nodes []graph.GraphNode, edges []graph.GraphEdge) {
	fmt.Fprintln(w, "graph LR")

	// Build node ID map from URL → short readable ID
	nodeID := make(map[string]string)
	counter := 0
	nodeLabel := func(rawURL string) string {
		if id, ok := nodeID[rawURL]; ok {
			return id
		}
		counter++
		id := fmt.Sprintf("N%d", counter)
		nodeID[rawURL] = id
		return id
	}

	// Declare nodes with path labels
	for _, n := range nodes {
		id := nodeLabel(n.URL)
		label := urlPath(n.URL)
		if label == "" || label == "/" {
			label = "/"
		}
		// Mermaid node: N1["/login"]
		fmt.Fprintf(w, "    %s[\"%s\"]\n", id, escapeMermaid(label))
	}

	// Draw edges (limit to prevent huge graphs, deduplicate by from→to)
	maxEdges := 50
	edgeCount := 0
	seenEdge := make(map[string]bool)
	for _, e := range edges {
		if edgeCount >= maxEdges {
			fmt.Fprintf(w, "    style_note[\"... +%d more edges\"]\n", len(edges)-maxEdges)
			break
		}
		fromID := nodeLabel(e.From)
		toID := nodeLabel(e.To)
		if fromID == toID {
			continue
		}
		edgeKey := fromID + "->" + toID
		if seenEdge[edgeKey] {
			continue
		}
		seenEdge[edgeKey] = true

		label := e.Action
		if len(label) > 30 {
			label = label[:27] + "..."
		}
		if label != "" {
			fmt.Fprintf(w, "    %s -->|\"%s\"| %s\n", fromID, escapeMermaid(label), toID)
		} else {
			fmt.Fprintf(w, "    %s --> %s\n", fromID, toID)
		}
		edgeCount++
	}
}

type siteTreeNode struct {
	name     string
	children map[string]*siteTreeNode
	isLeaf   bool
	count    int // number of instances (>1 for collapsed UUID paths)
}

// renderSiteTree outputs an indented tree showing the URL path hierarchy.
// Only shows paths for the target host. Query params are stripped.
// Templated segments (UUIDs, session tokens) are collapsed with instance counts.
func renderSiteTree(w io.Writer, endpoints []Endpoint, targetURL string) {
	var targetHost string
	if u, err := url.Parse(targetURL); err == nil {
		targetHost = u.Hostname()
	}

	// Count occurrences of each normalized path (UUIDs/tokens replaced)
	pathCounts := make(map[string]int)
	for _, ep := range endpoints {
		// Skip static assets from site tree
		if IsStaticAsset(ep.URL) {
			continue
		}
		u, err := url.Parse(ep.URL)
		if err != nil {
			continue
		}
		// Only include target host paths
		if targetHost != "" && u.Hostname() != targetHost {
			continue
		}
		// Use just the path (no query params) to avoid duplicates like
		// /task?_rsc=3hm2o and /task?_rsc=dtgoq showing as separate entries
		p := u.Path
		if p == "" {
			continue
		}
		// Replace UUIDs and session tokens to collapse templated paths
		normalized := replaceTemplatedSegments(p)
		pathCounts[normalized]++
	}

	paths := make([]string, 0, len(pathCounts))
	for p := range pathCounts {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	root := &siteTreeNode{name: "/", children: make(map[string]*siteTreeNode), count: 0}

	for _, p := range paths {
		parts := strings.Split(strings.Trim(p, "/"), "/")
		current := root
		for _, part := range parts {
			if part == "" {
				continue
			}
			if current.children[part] == nil {
				current.children[part] = &siteTreeNode{name: part, children: make(map[string]*siteTreeNode)}
			}
			current = current.children[part]
		}
		current.isLeaf = true
		current.count = pathCounts[p]
	}

	fmt.Fprintf(w, "/\n")
	renderTreeNode(w, root, "")
}

func renderTreeNode(w io.Writer, node *siteTreeNode, prefix string) {
	// Sort children
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Strings(names)

	for i, name := range names {
		child := node.children[name]
		isLast := i == len(names)-1

		connector := "├── "
		childPrefix := prefix + "│   "
		if isLast {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		label := name
		if len(child.children) == 0 && child.isLeaf {
			label = name // leaf
		} else if len(child.children) > 0 {
			label = name + "/" // directory
		}

		// Show instance count for collapsed UUID paths
		if child.count > 1 {
			label = fmt.Sprintf("%s (%d instances)", label, child.count)
		}

		fmt.Fprintf(w, "%s%s%s\n", prefix, connector, label)
		renderTreeNode(w, child, childPrefix)
	}
}

// urlPath extracts just the path from a URL.
func urlPath(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}
	p := u.Path
	if u.RawQuery != "" {
		p += "?" + u.RawQuery
	}
	return p
}

// isJSURL returns true if the URL points to a JavaScript file.
func isJSURL(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	p := strings.ToLower(u.Path)
	return strings.HasSuffix(p, ".js") || strings.HasSuffix(p, ".mjs")
}

// escapeMermaid escapes characters that break Mermaid syntax.
func escapeMermaid(s string) string {
	s = strings.ReplaceAll(s, `"`, `#quot;`)
	s = strings.ReplaceAll(s, `<`, `#lt;`)
	s = strings.ReplaceAll(s, `>`, `#gt;`)
	return s
}
