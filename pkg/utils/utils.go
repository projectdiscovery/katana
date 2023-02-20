package utils

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lukasbob/srcset"
	errorutil "github.com/projectdiscovery/utils/errors"
)

// IsURL returns true if a provided string is URL
func IsURL(url string) bool {
	return strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://")
}

// ParseSRCSetTag parses srcset tag returning found URLs
func ParseSRCSetTag(value string) []string {
	set := srcset.Parse(value)
	values := make([]string, 0, len(set))
	for _, item := range set {
		values = append(values, item.URL)
	}
	return values
}

// ParseLinkTag parses link tag values returning found urls
//
// Inspired from: https://github.com/tomnomnom/linkheader
func ParseLinkTag(value string) []string {
	urls := make([]string, 0)

	for _, chunk := range strings.Split(value, ",") {
		for _, piece := range strings.Split(chunk, ";") {
			piece = strings.Trim(piece, " ")
			if piece == "" {
				continue
			}
			if piece[0] == '<' && piece[len(piece)-1] == '>' {
				urls = append(urls, strings.Trim(piece, "<>"))
				continue
			}
		}
	}
	return urls
}

// ParseRefreshTag parses refresh tag values returning found urls
func ParseRefreshTag(value string) string {
	chunks := strings.Split(value, "url=")
	if len(chunks) < 2 {
		return ""
	}
	chunk := chunks[1]
	chunk = strings.TrimSuffix(chunk, ";")
	if chunk == "" {
		return ""
	}
	return chunk
}

// WebUserAgent returns the chrome-web user agent
func WebUserAgent() string {
	return "Mozilla/5.0 (Macintosh; Intel Mac OS X 11_1) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.88 Safari/537.36"
}

func GetDefaultCustomConfigFile() (string, error) {
	homedir, err := os.UserHomeDir()
	if err != nil {
		return "", errorutil.NewWithTag("customfield", "could not get home directory").Wrap(err)
	}
	defaultConfig := filepath.Join(homedir, ".config", "katana", "field-config.yaml")
	return defaultConfig, nil
}
