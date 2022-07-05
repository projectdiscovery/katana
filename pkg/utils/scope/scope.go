package scope

import (
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"

	"go.uber.org/multierr"
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

	var domain, subdomain string

	parsedIP := net.ParseIP(hostname)
	parsedDomain, errDomainParse := publicsuffix.EffectiveTLDPlusOne(hostname)

	// The target can be:
	// - IP
	// - RFC Domain Name
	// - Resolvable domain name via /etc/hosts or resolver
	switch {
	case parsedIP != nil:
		domain = hostname
	case parsedDomain != "" && errDomainParse == nil:
		domain = parsedDomain
		subdomain = strings.TrimSuffix(hostname, domain)
	default:
		// attempt to resolve the domain
		addrs, errLookupHostname := net.LookupHost(hostname)
		// consider the hardcoded hostname as a valid endpoint
		if len(addrs) > 0 && errLookupHostname == nil {
			domain = hostname
		} else {
			return false, fmt.Errorf("domain '%s' could not be parsed/resolved: %s:", hostname, multierr.Combine(errDomainParse, errLookupHostname))
		}
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
