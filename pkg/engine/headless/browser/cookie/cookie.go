// Package cookie implements cookie consent handling for the
// crawler. Its a partial port of I-Still-Dont-Care-About-Cookies.
package cookie

import (
	_ "embed"
	"encoding/json"
	"sort"
	"strings"

	"github.com/go-rod/rod/lib/proto"
)

type CookieConsentBlockRequest struct {
	ID        int       `json:"id"`
	Priority  int       `json:"priority"`
	Condition Condition `json:"condition,omitempty"`
}

type Action struct {
	Type string `json:"type"`
}

type Condition struct {
	URLFilter                string   `json:"urlFilter"`
	ResourceTypes            []string `json:"resourceTypes"`
	InitiatorDomains         []string `json:"initiatorDomains"`
	ExcludedInitiatorDomains []string `json:"excludedInitiatorDomains"`
}

//go:embed rules.json
var rules []byte

var cookieConsentBlockRequests []CookieConsentBlockRequest

func init() {
	err := json.Unmarshal(rules, &cookieConsentBlockRequests)
	if err != nil {
		panic(err)
	}

	sort.SliceStable(cookieConsentBlockRequests, func(i, j int) bool {
		return cookieConsentBlockRequests[i].Priority > cookieConsentBlockRequests[j].Priority
	})
}

// ShouldBlockRequest determines if a request should be blocked based on cookie consent rules
func ShouldBlockRequest(url string, resourceType proto.NetworkResourceType, initiatorDomain string) bool {
	resourceTypeStr := getResourceType(resourceType)
	for _, rule := range cookieConsentBlockRequests {
		if matchesRule(rule, url, resourceTypeStr, initiatorDomain) {
			return true
		}
	}
	return false
}

// matchesRule checks if a request matches a specific cookie consent block rule
func matchesRule(rule CookieConsentBlockRequest, url string, resourceType string, initiatorDomain string) bool {
	if !strings.Contains(url, rule.Condition.URLFilter) {
		return false
	}

	if len(rule.Condition.ResourceTypes) > 0 {
		matched := false
		for _, rt := range rule.Condition.ResourceTypes {
			if rt == resourceType {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(rule.Condition.InitiatorDomains) > 0 {
		matched := false
		for _, domain := range rule.Condition.InitiatorDomains {
			if strings.Contains(initiatorDomain, domain) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	if len(rule.Condition.ExcludedInitiatorDomains) > 0 {
		for _, domain := range rule.Condition.ExcludedInitiatorDomains {
			if strings.Contains(initiatorDomain, domain) {
				return false
			}
		}
	}

	return true
}

func getResourceType(resourceType proto.NetworkResourceType) string {
	switch resourceType {
	case proto.NetworkResourceTypeStylesheet:
		return "stylesheet"
	case proto.NetworkResourceTypeScript:
		return "script"
	case proto.NetworkResourceTypeImage:
		return "image"
	case proto.NetworkResourceTypeFont:
		return "font"
	case proto.NetworkResourceTypeXHR:
		return "xmlhttprequest"
	case proto.NetworkResourceTypePing:
		return "ping"
	case proto.NetworkResourceTypeCSPViolationReport:
		return "csp_report"
	case proto.NetworkResourceTypeMedia:
		return "media"
	case proto.NetworkResourceTypeWebSocket:
		return "websocket"
	case proto.NetworkResourceTypeOther:
		return "other"
	}
	return string(resourceType)
}
