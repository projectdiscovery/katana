package extensions

import (
	"path"
	"strings"

	"github.com/projectdiscovery/gologger"
	urlutil "github.com/projectdiscovery/utils/url"
)

// defaultDenylist is the default list of extensions to be denied
var defaultDenylist = []string{".3g2", ".3gp", ".7z", ".apk", ".arj", ".avi", ".axd", ".bmp", ".csv", ".deb", ".dll", ".doc", ".drv", ".eot", ".exe", ".flv", ".gif", ".gifv", ".gz", ".h264", ".ico", ".iso", ".jar", ".jpeg", ".jpg", ".lock", ".m4a", ".m4v", ".map", ".mkv", ".mov", ".mp3", ".mp4", ".mpeg", ".mpg", ".msi", ".ogg", ".ogm", ".ogv", ".otf", ".pdf", ".pkg", ".png", ".ppt", ".psd", ".rar", ".rm", ".rpm", ".svg", ".swf", ".sys", ".tar.gz", ".tar", ".tif", ".tiff", ".ttf", ".txt", ".vob", ".wav", ".webm", ".webp", ".wmv", ".woff", ".woff2", ".xcf", ".xls", ".xlsx", ".zip"}

// Validator is a validator for file extension
type Validator struct {
	extensionsMatch  map[string]struct{}
	extensionsFilter map[string]struct{}
}

// NewValidator creates a new extension validator instance
func NewValidator(extensionsMatch, extensionsFilter []string) *Validator {
	validator := &Validator{
		extensionsMatch:  make(map[string]struct{}),
		extensionsFilter: make(map[string]struct{}),
	}

	for _, extension := range extensionsMatch {
		normalized := normalizeExtension(extension)
		validator.extensionsMatch[normalized] = struct{}{}
		gologger.Debug().Msgf("Added match extension: %s -> %s", extension, normalized)
	}
	for _, item := range defaultDenylist {
		normalized := normalizeExtension(item)
		validator.extensionsFilter[normalized] = struct{}{}
		gologger.Debug().Msgf("Added default deny extension: %s -> %s", item, normalized)
	}
	for _, extension := range extensionsFilter {
		normalized := normalizeExtension(extension)
		validator.extensionsFilter[normalized] = struct{}{}
		gologger.Debug().Msgf("Added filter extension: %s -> %s", extension, normalized)
	}
	return validator
}

// ExactMatch returns true if a path matches the extension match list exactly
func (e *Validator) ExactMatch(item string) bool {
	if len(e.extensionsMatch) == 0 {
		return true
	}

	var extension string
	u, err := urlutil.Parse(item)
	if err != nil {
		gologger.Warning().Msgf("exactmatch: failed to parse url %v got %v", item, err)
		return false
	}

	cleanPath := strings.TrimRight(u.Path, "/")
	if cleanPath != "" {
		extension = strings.ToLower(path.Ext(cleanPath))
	} else {
		extension = strings.ToLower(path.Ext(strings.TrimRight(item, "/")))
	}

	// Only match if there's an extension and it's in our match list
	if extension != "" {
		if _, ok := e.extensionsMatch[extension]; ok {
			return true
		}
	}
	return false
}

// ValidatePath returns true if an extension is allowed by the validator
func (e *Validator) ValidatePath(item string) bool {
	// Handle local paths directly if they don't look like URLs
	if !strings.Contains(item, "://") {
		cleanPath := strings.TrimRight(item, "/")
		extension := normalizeExtension(path.Ext(cleanPath))

		// No extension case
		if extension == "" {
			return true
		}

		// Check extension matches first - if an extension is in the match list, always allow it
		if len(e.extensionsMatch) > 0 {
			if _, ok := e.extensionsMatch[extension]; ok {
				return true
			}
			return true // allow non-matching extensions for crawling
		}

		// If no extension matches defined, check deny list
		if _, ok := e.extensionsFilter[extension]; ok {
			gologger.Debug().Msgf("Extension %s found in deny list", extension)
			return false
		}

		// Allow anything not in deny list when no matches are defined
		return true
	}

	// Handle URLs
	u, err := urlutil.Parse(item)
	if err != nil {
		gologger.Warning().Msgf("validatepath: failed to parse url %v got %v", item, err)
		return false
	}

	// Always allow root domains and directory paths
	if u.Path == "" || strings.HasSuffix(u.Path, "/") {
		return true
	}

	// For URLs, allow everything except if there are specific extension matches
	if len(e.extensionsMatch) > 0 {
		extension := normalizeExtension(path.Ext(u.Path))
		if extension == "" {
			return true // allow paths without extensions
		}
		// Only check for matches, ignore deny list for URLs
		if _, ok := e.extensionsMatch[extension]; ok {
			return true
		}
		return true // allow non-matching extensions for crawling
	}
	return true
}

func normalizeExtension(extension string) string {
	// Make sure we have a clean extension with exactly one leading dot
	extension = strings.ToLower(extension)
	extension = strings.TrimLeft(extension, ".")
	return "." + extension
}
