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
	fieldScope        dnsScopeField
	fieldScopePattern *regexp.Regexp
}

type dnsScopeField int

const (
	dnDnsScopeField dnsScopeField = iota + 1
	rdnDnsScopeField
	fqdnDNSScopeField
	customDNSScopeField
)

var stringToDNSScopeField = map[string]dnsScopeField{
	"dn":   dnDnsScopeField,
	"rdn":  rdnDnsScopeField,
	"fqdn": fqdnDNSScopeField,
}

// NewManager returns a new scope manager for crawling
func NewManager(inScope, outOfScope []string, fieldScope string, noScope bool) (*Manager, error) {
	manager := &Manager{
		noScope: noScope,
	}

	if scopeValue, ok := stringToDNSScopeField[fieldScope]; !ok {
		manager.fieldScope = customDNSScopeField
		if compiled, err := regexp.Compile(fieldScope); err != nil {
			return nil, fmt.Errorf("could not compile regex %s: %s", fieldScope, err)
		} else {
			manager.fieldScopePattern = compiled
		}
	} else {
		manager.fieldScope = scopeValue
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
	if m.noScope {
		return true, nil
	}
	hostname := URL.Hostname()

	// Validate host if not explicitly disabled by the user
	dnsValidated, err := m.validateDNS(hostname, rootHostname)
	if err != nil {
		return false, err
	}
	// If we have URL rules also consider them
	if len(m.inScope) > 0 || len(m.outOfScope) > 0 {
		urlValidated, err := m.validateURL(URL.String())
		if err != nil {
			return false, err
		}
		if urlValidated && dnsValidated {
			return true, nil
		}
		return false, nil
	}
	return dnsValidated, nil
}

func (m *Manager) validateURL(URL string) (bool, error) {
	for _, item := range m.outOfScope {
		if item.MatchString(URL) {
			return false, nil
		}
	}
	if len(m.inScope) == 0 {
		return true, nil
	}

	var inScopeMatched bool
	for _, item := range m.inScope {
		if item.MatchString(URL) {
			inScopeMatched = true
			break
		}
	}
	return inScopeMatched, nil
}

func (m *Manager) validateDNS(hostname, rootHostname string) (bool, error) {
	parsed := net.ParseIP(hostname)
	if m.fieldScope == customDNSScopeField {
		// If we have a custom regex, we need to match it against the full hostname
		if m.fieldScopePattern.MatchString(hostname) {
			return true, nil
		}
	}
	if m.fieldScope == fqdnDNSScopeField || parsed != nil {
		matched := strings.EqualFold(hostname, rootHostname)
		return matched, nil
	}

	rdn, dn, err := getDomainRDNandRDN(rootHostname)
	if err != nil {
		return false, err
	}
	switch m.fieldScope {
	case dnDnsScopeField:
		return strings.Contains(hostname, dn), nil
	case rdnDnsScopeField:
		return strings.HasSuffix(hostname, rdn), nil
	}
	return false, nil
}

func getDomainRDNandRDN(domain string) (string, string, error) {
	if strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") || strings.Contains(domain, "..") {
		return "", "", fmt.Errorf("publicsuffix: empty label in domain %q", domain)
	}
	suffix, _ := publicsuffix.PublicSuffix(domain)
	if len(domain) <= len(suffix) {
		return domain, "", nil
	}
	i := len(domain) - len(suffix) - 1
	if domain[i] != '.' {
		return domain, "", nil
	}
	return domain[1+strings.LastIndex(domain[:i], "."):], domain[1+strings.LastIndex(domain[:i], ".") : len(domain)-len(suffix)-1], nil
}
