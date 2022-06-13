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
	includeSubdomains bool
}

// NewManager returns a new scope manager for crawling
func NewManager(inScope, outOfScope []string, includeSubdomains bool) (*Manager, error) {
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
	return manager, nil
}

// Validate returns true if the URL matches scope rules
func (m *Manager) Validate(URL *url.URL) (bool, error) {
	hostname := URL.Hostname()

	var subdomain string
	var domain string
	parsed := net.ParseIP(hostname)
	if parsed != nil {
		domain = hostname
	} else {
		var err error
		domain, err = publicsuffix.EffectiveTLDPlusOne(hostname)
		if err != nil {
			return false, fmt.Errorf("could not parse domain %s: %s", hostname, err)
		}
		subdomain = strings.TrimSuffix(hostname, domain)
	}

	if len(m.outOfScope) > 0 {
		var outOfScopeMatched bool
		for _, item := range m.outOfScope {
			if item.MatchString(domain) {
				outOfScopeMatched = true
			}
		}
		if outOfScopeMatched {
			return false, nil
		}
	}
	if len(m.inScope) > 0 {
		var inScopeMatched bool
		for _, item := range m.inScope {
			if item.MatchString(domain) {
				inScopeMatched = true
			}
		}
		if !inScopeMatched {
			return false, nil
		}
		if subdomain != "" {
			if m.includeSubdomains {
				return true, nil
			}
			return false, nil
		}
		return true, nil
	}
	return true, nil
}
