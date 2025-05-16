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
		validator.extensionsMatch[normalizeExtension(extension)] = struct{}{}
	}
	for _, item := range defaultDenylist {
		validator.extensionsFilter[normalizeExtension(item)] = struct{}{}
	}
	for _, extension := range extensionsFilter {
		validator.extensionsFilter[normalizeExtension(extension)] = struct{}{}
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
	u, err := urlutil.Parse(item)
	if err != nil {
		gologger.Warning().Msgf("validatepath: failed to parse url %v got %v", item, err)
		return false
	}

	// Clean the path by trimming trailing slashes
	cleanPath := strings.TrimRight(u.Path, "/")

	// Always allow root domains and directory paths
	if cleanPath == "" || strings.HasSuffix(u.Path, "/") {
		return true
	}

	// Get the extension from the clean path
	extension := strings.ToLower(path.Ext(cleanPath))

	// Handle paths without extensions
	if extension == "" {
		return len(e.extensionsMatch) == 0 // Allow if no extension matching is enabled
	}

	// If we have extension matches defined, only allow those extensions
	if len(e.extensionsMatch) > 0 {
		if _, ok := e.extensionsMatch[extension]; ok {
			return true
		}
		return false
	}

	// Otherwise use the extension filter list
	if _, ok := e.extensionsFilter[extension]; ok {
		return false
	}
	return true
}

func normalizeExtension(extension string) string {
	extension = strings.ToLower(extension)
	if strings.HasPrefix(extension, ".") {
		return extension
	}
	return "." + extension
}
