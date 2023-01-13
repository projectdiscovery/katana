package extensions

import (
	"fmt"
	"net/url"
	"path"
	"strings"
)

// defaultDenylist is the default list of extensions to be denied
var defaultDenylist = []string{".3g2", ".3gp", ".7z", ".apk", ".arj", ".avi", ".axd", ".bmp", ".csv", ".deb", ".dll", ".doc", ".drv", ".eot", ".exe", ".flv", ".gif", ".gifv", ".gz", ".h264", ".ico", ".iso", ".jar", ".jpeg", ".jpg", ".lock", ".m4a", ".m4v", ".map", ".mkv", ".mov", ".mp3", ".mp4", ".mpeg", ".mpg", ".msi", ".ogg", ".ogm", ".ogv", ".otf", ".pdf", ".pkg", ".png", ".ppt", ".psd", ".rar", ".rm", ".rpm", ".svg", ".swf", ".sys", ".tar.gz", ".tar", ".tif", ".tiff", ".ttf", ".txt", ".vob", ".wav", ".webm", ".wmv", ".woff", ".woff2", ".xcf", ".xls", ".xlsx", ".zip"}

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

	extensionNormalize := func(extension string) string {
		extension = strings.ToLower(extension)
		if !strings.HasPrefix(extension, ".") {
			return fmt.Sprintf(".%s", extension)
		}
		return extension
	}

	for _, extension := range extensionsMatch {
		validator.extensionsMatch[extensionNormalize(extension)] = struct{}{}
	}
	for _, item := range defaultDenylist {
		validator.extensionsFilter[extensionNormalize(item)] = struct{}{}
	}
	for _, extension := range extensionsFilter {
		validator.extensionsFilter[extensionNormalize(extension)] = struct{}{}
	}
	return validator
}

// ValidatePath returns true if an extension is allowed by the validator
func (e *Validator) ValidatePath(item string) bool {
	var extension string
	u, _ := url.Parse(item)
	if u != nil {
		extension = strings.ToLower(path.Ext(u.Path))
	} else {
		extension = strings.ToLower(path.Ext(item))
	}
	if extension == "" && len(e.extensionsMatch) > 0 {
		return false
	}
	if len(e.extensionsMatch) > 0 {
		if _, ok := e.extensionsMatch[extension]; ok {
			return true
		}
		return false
	}
	if _, ok := e.extensionsFilter[extension]; ok {
		return false
	}
	return true
}
