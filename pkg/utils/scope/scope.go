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

// Validate returns true if the URL matches scope rules.
// When noScope is true, DNS validation is skipped but URL-based scope rules still apply.
func (m *Manager) Validate(URL *url.URL, rootHostname string) (bool, error) {
	if !m.noScope {
		// Only validate DNS if scope is enabled
		hostname := URL.Hostname()
		dnsValidated, err := m.validateDNS(hostname, rootHostname)
		if err != nil || !dnsValidated {
			return false, err
		}
	}

	if len(m.inScope) > 0 || len(m.outOfScope) > 0 {
		urlValidated, err := m.validateURL(URL.String())
		if err != nil || !urlValidated {
			return false, err
		}
	}

	return true, nil
}

// validateURL checks whether the given URL matches the configured inScope and outOfScope patterns.
// It returns true if the URL is allowed (matches inScope and doesn't match outOfScope),
// false if rejected, and an error if pattern matching fails.
// When both inScope and outOfScope are empty, it returns true with no error.
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

// validateDNS performs DNS-based scope validation by checking if the URL's hostname
// matches the configured host-based scope rules. It returns true if the hostname
// is within scope, false if out of scope, and an error if DNS resolution or
// validation fails.
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

// getDomainRDNandRDN extracts and returns the root domain name (RDN) and the
// effective top-level domain plus one label (eTLD+1) from the given hostname.
// It returns empty strings and an error if the hostname cannot be parsed.
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
