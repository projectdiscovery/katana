package hybrid

import (
	"fmt"
	"strings"
	"sync"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// cookieStore shares cookies between browser agents. Each agent is its own
// Chrome process, so without it a session cookie that one agent gets, for
// example after a login, is missing from the pages that the other agents
// crawl. Before a navigation an agent sets the shared cookies into its
// browser. After the navigation, it writes back the cookies that changed in
// its browser, from Set-Cookie headers or from scripts, and only then queues
// the links of the page, so the agent that takes a link has the cookies.
// Chrome does not show Set-Cookie headers to the Fetch hijack, so the browser
// is the source. Only changes are written, so an agent with an old copy of a
// cookie does not overwrite a newer one. It is used only with more than one
// agent; one agent keeps its cookies in its own browser, as before.
type cookieStore struct {
	mu      sync.Mutex
	cookies map[cookieKey]*proto.NetworkCookieParam
}

// cookieKey identifies a cookie as a browser does: by name, domain, path and
// partition. A partitioned cookie (CHIPS) is apart from an unpartitioned one
// with the same name, domain and path.
type cookieKey struct {
	name, domain, path string
	partition          string
}

func newCookieKey(c *proto.NetworkCookie) cookieKey {
	key := cookieKey{name: c.Name, domain: c.Domain, path: c.Path}
	if p := c.PartitionKey; p != nil {
		key.partition = fmt.Sprintf("%s|%t", p.TopLevelSite, p.HasCrossSiteAncestor)
	}
	return key
}

func newCookieStore() *cookieStore {
	return &cookieStore{cookies: map[cookieKey]*proto.NetworkCookieParam{}}
}

// apply sets the shared cookies into the browser context of b. A cookie that
// an agent deleted is set as expired, so b deletes it too.
func (s *cookieStore) apply(b *rod.Browser) error {
	s.mu.Lock()
	params := make([]*proto.NetworkCookieParam, 0, len(s.cookies))
	for _, p := range s.cookies {
		params = append(params, p)
	}
	s.mu.Unlock()
	if len(params) == 0 {
		return nil
	}
	return proto.StorageSetCookies{Cookies: params, BrowserContextID: b.BrowserContextID}.Call(b)
}

// record writes to the store the cookies that changed between before and
// after: new or changed cookies are set, and missing ones are deleted.
func (s *cookieStore) record(before, after map[cookieKey]*proto.NetworkCookie) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, cookie := range after {
		if old, ok := before[key]; ok && sameCookie(old, cookie) {
			continue
		}
		s.cookies[key] = cookieParam(cookie)
	}
	for key, cookie := range before {
		if _, ok := after[key]; ok {
			continue
		}
		deleted := cookieParam(cookie)
		deleted.Value = ""
		deleted.Expires = 1 // in the past, so a browser deletes the cookie
		s.cookies[key] = deleted
	}
}

// browserCookies returns the cookies in the browser context of b.
func browserCookies(b *rod.Browser) (map[cookieKey]*proto.NetworkCookie, error) {
	res, err := proto.StorageGetCookies{BrowserContextID: b.BrowserContextID}.Call(b)
	if err != nil {
		return nil, err
	}
	cookies := make(map[cookieKey]*proto.NetworkCookie, len(res.Cookies))
	for _, c := range res.Cookies {
		cookies[newCookieKey(c)] = c
	}
	return cookies, nil
}

func sameCookie(a, b *proto.NetworkCookie) bool {
	return a.Value == b.Value && a.Expires == b.Expires && a.Session == b.Session &&
		a.Secure == b.Secure && a.HTTPOnly == b.HTTPOnly && a.SameSite == b.SameSite
}

func cookieParam(c *proto.NetworkCookie) *proto.NetworkCookieParam {
	port := c.SourcePort
	p := &proto.NetworkCookieParam{
		Name:         c.Name,
		Value:        c.Value,
		Domain:       c.Domain,
		Path:         c.Path,
		Secure:       c.Secure,
		HTTPOnly:     c.HTTPOnly,
		SameSite:     c.SameSite,
		Priority:     c.Priority,
		SourceScheme: c.SourceScheme,
		SourcePort:   &port,
	}
	if c.PartitionKey != nil {
		partition := *c.PartitionKey
		p.PartitionKey = &partition
	}
	// A host-only cookie has a domain without a leading dot. Set it by URL,
	// as a domain would also send it to subdomains.
	if c.Domain != "" && !strings.HasPrefix(c.Domain, ".") {
		scheme := "http"
		if c.Secure {
			scheme = "https"
		}
		p.URL = scheme + "://" + c.Domain + c.Path
		p.Domain = ""
	}
	// A session cookie has no expiry; -1 would set it as already expired.
	if !c.Session {
		p.Expires = c.Expires
	}
	return p
}
