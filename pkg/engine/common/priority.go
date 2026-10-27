package common

import (
	"net/url"
	"strings"
	"unicode"

	"github.com/projectdiscovery/katana/pkg/navigation"
	"github.com/projectdiscovery/katana/pkg/utils/queue"
)

// surfaceWords are path segments that usually lead to more attack surface
// than an average page, so the priority strategy visits them first.
var surfaceWords = map[string]struct{}{
	"admin": {}, "administrator": {}, "api": {}, "graphql": {}, "rest": {},
	"upload": {}, "uploads": {}, "file": {}, "files": {}, "download": {},
	"import": {}, "export": {}, "backup": {},
	"auth": {}, "login": {}, "signin": {}, "signup": {}, "register": {},
	"oauth": {}, "sso": {}, "token": {}, "password": {}, "reset": {},
	"account": {}, "user": {}, "users": {}, "profile": {},
	"dashboard": {}, "manage": {}, "console": {}, "config": {}, "settings": {},
	"debug": {}, "internal": {},
}

// priority returns the queue priority of a request, lower is visited first.
// With the priority strategy each score point is worth one level of depth,
// so a promising link a level deeper still goes ahead of a plain one.
func (s *Shared) priority(nr *navigation.Request) int {
	if s.Options.Options.Strategy != queue.Priority.String() {
		return nr.Depth
	}
	return nr.Depth - surfaceScore(nr)
}

func surfaceScore(nr *navigation.Request) int {
	parsed, err := url.Parse(nr.URL)
	if err != nil {
		return 0
	}
	score := 0
	for _, word := range strings.FieldsFunc(strings.ToLower(parsed.Path), isWordSeparator) {
		if _, ok := surfaceWords[word]; ok {
			score += 2
			break
		}
	}
	if parsed.RawQuery != "" {
		score++
	}
	if nr.Method != "" && nr.Method != "GET" {
		score++
	}
	return score
}

func isWordSeparator(r rune) bool {
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
