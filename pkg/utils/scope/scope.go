package scope

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/publicsuffix"
)

// Manager manages scope for crawling process
type Manager struct {
	inScope           []*regexp.Regexp
	outOfScope        []*regexp.Regexp
	inScopeDomains    []*regexp.Regexp
	outOfScopeDomains []*regexp.Regexp
	includeSubdomains bool
}

// NewManager returns a new scope manager for crawling
func NewManager(inScope, outOfScope, inScopeDomains, outOfScopeDomains []string, includeSubdomains bool) (*Manager, error) {
	manager := &Manager{includeSubdomains: includeSubdomains}

	for _, regex := range inScope {
		if compiled, err := regexp.Compile(regex); err != nil {
			return nil, fmt.Errorf("could not compile regex %s: %s", regex, err)
		} else {
			manager.inScope = append(manager.inScope, compiled)
		}
	}
	for _, regex := range outOfScope {
		if compiled, err := regexp.Compile(regex); err != nil {
			return nil, fmt.Errorf("could not compile regex %s: %s", regex, err)
		} else {
			manager.outOfScope = append(manager.outOfScope, compiled)
		}
	}
	for _, regex := range inScopeDomains {
		if compiled, err := regexp.Compile(regex); err != nil {
			return nil, fmt.Errorf("could not compile regex %s: %s", regex, err)
		} else {
			manager.inScopeDomains = append(manager.inScopeDomains, compiled)
		}
	}
	for _, regex := range outOfScopeDomains {
		if compiled, err := regexp.Compile(regex); err != nil {
			return nil, fmt.Errorf("could not compile regex %s: %s", regex, err)
		} else {
			manager.outOfScopeDomains = append(manager.outOfScopeDomains, compiled)
		}
	}
	return manager, nil
}

// Validate returns true if the URL matches scope rules
func (m *Manager) Validate(URL *url.URL) (bool, error) {
	hostname := URL.Hostname()

	// If we have URL rules also consider them, if not just work on domain scope
	inScopeURLs := len(m.inScope) > 0 && len(m.outOfScope) > 0

	var validated, defaultMatch bool
	var err error
	if len(m.inScopeDomains) > 0 || len(m.outOfScopeDomains) > 0 {
		validated, defaultMatch, err = m.validateHostname(hostname)
		if !inScopeURLs {
			return validated, err
		}
		if !defaultMatch {
			return validated, err
		}
	}
	validated, err = m.validateURL(URL.String())
	return validated, err
}

func (m *Manager) validateURL(URL string) (bool, error) {
	for _, item := range m.outOfScope {
		if item.MatchString(URL) {
			return false, nil
		}
	}

	var inScopeMatched bool
	if len(m.inScope) > 0 {
		for _, item := range m.inScope {
			if item.MatchString(URL) {
				inScopeMatched = true
				break
			}
		}
		if !inScopeMatched {
			return false, nil
		}
	}
	return true, nil
}

func (m *Manager) validateHostname(hostname string) (bool, bool, error) {
	var subdomain, domain string
	var err error

	parsed := net.ParseIP(hostname)
	if parsed == nil {
		domain, err = publicsuffix.EffectiveTLDPlusOne(hostname)
		if err != nil {
			return false, false, fmt.Errorf("could not parse domain %s: %s", hostname, err)
		}
		subdomain = strings.TrimSuffix(hostname, domain)
	} else {
		domain = hostname
	}

	for _, item := range m.outOfScopeDomains {
		if item.MatchString(domain) {
			return false, false, nil
		}
	}
	var inScopeMatched bool
	for _, item := range m.inScopeDomains {
		if item.MatchString(domain) {
			inScopeMatched = true
			break
		}
	}
	if !inScopeMatched {
		return false, false, nil
	}
	if subdomain != "" {
		if m.includeSubdomains {
			return true, false, nil
		}
		return false, false, nil
	}
	return true, true, nil
}
