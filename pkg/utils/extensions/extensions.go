package extensions

import (
	"fmt"
	"path"
	"strings"
)

// defaultDenylist is the default list of extensions to be denied
var defaultDenylist = []string{".3g2", ".3gp", ".7z", ".apk", ".arj", ".avi", ".axd", ".bmp", ".csv", ".deb", ".dll", ".doc", ".drv", ".eot", ".exe", ".flv", ".gif", ".gifv", ".gz", ".h264", ".ico", ".iso", ".jar", ".jpeg", ".jpg", ".lock", ".m4a", ".m4v", ".map", ".mkv", ".mov", ".mp3", ".mp4", ".mpeg", ".mpg", ".msi", ".ogg", ".ogm", ".ogv", ".otf", ".pdf", ".pkg", ".png", ".ppt", ".psd", ".rar", ".rm", ".rpm", ".svg", ".swf", ".sys", ".tar.gz", ".tar", ".tif", ".tiff", ".ttf", ".txt", ".vob", ".wav", ".webm", ".wmv", ".woff", ".woff2", ".xcf", ".xls", ".xlsx", ".zip"}

// Validator is a validator for file extension
type Validator struct {
	allExtensions  bool
	extensions     map[string]struct{}
	extensionsDeny map[string]struct{}
}

// NewValidator creates a new extension validator instance
func NewValidator(extensions, extensionsAllowlist, extensionsDenyList []string) *Validator {
	validator := &Validator{
		extensions:     make(map[string]struct{}),
		extensionsDeny: make(map[string]struct{}),
	}

	extensionNormalize := func(extension string) string {
		if !strings.HasPrefix(extension, ".") {
			return fmt.Sprintf(".%s", extension)
		}
		return extension
	}
	for _, extension := range extensions {
		if extension == "*" {
			validator.allExtensions = true
		} else {
			validator.extensions[extensionNormalize(extension)] = struct{}{}
		}
	}
	for _, extension := range defaultDenylist {
		validator.extensionsDeny[extensionNormalize(extension)] = struct{}{}
	}
	for _, extension := range extensionsDenyList {
		validator.extensionsDeny[extensionNormalize(extension)] = struct{}{}
	}
	for _, extension := range extensionsAllowlist {
		delete(validator.extensionsDeny, extensionNormalize(extension))
	}
	return validator
}

// ValidatePath returns true if an extension is allowed by the validator
func (e *Validator) ValidatePath(item string) bool {
	extension := path.Ext(item)
	if extension == "" {
		return true
	}
	if len(e.extensions) > 0 && !e.allExtensions {
		if _, ok := e.extensions[extension]; ok {
			return true
		}
		return false
	}
	if _, ok := e.extensionsDeny[extension]; ok {
		return false
	}
	return true
}
