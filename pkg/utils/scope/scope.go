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
	noScope           bool
	includeSubdomains bool
}

// NewManager returns a new scope manager for crawling
func NewManager(inScope, outOfScope []string, includeSubdomains, noScope bool) (*Manager, error) {
	manager := &Manager{
		noScope:           noScope,
		includeSubdomains: includeSubdomains,
	}

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
func (m *Manager) Validate(URL *url.URL, rootHostname string) (bool, error) {
	hostname := URL.Hostname()

	// Validate host if not explicitly disabled by the user
	if !m.noScope {
		if strings.EqualFold(hostname, rootHostname) {
			return true, nil
		}
		if validated, err := m.validateSubdomain(hostname, rootHostname); err != nil {
			return false, err
		} else if validated {
			return validated, nil
		}
	}

	// If we have URL rules also consider them
	if len(m.inScope) > 0 || len(m.outOfScope) > 0 {
		return m.validateURL(URL.String())
	}
	return false, nil
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

func (m *Manager) validateSubdomain(hostname, rootHostname string) (bool, error) {
	var subdomain, domain string
	var err error

	parsed := net.ParseIP(hostname)
	if parsed == nil {
		domain, err = publicsuffix.EffectiveTLDPlusOne(hostname)
		if err != nil {
			return false, fmt.Errorf("could not parse domain %s: %s", hostname, err)
		}
		subdomain = strings.TrimSuffix(hostname, domain)
	} else {
		domain = hostname
	}

	if m.includeSubdomains && subdomain != "" && strings.EqualFold(domain, rootHostname) {
		return true, nil
	}
	return false, nil
}
