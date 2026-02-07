//go:build !(386 || windows) && (!cgo || !jsluice)

package utils

// ExtractJsluiceEndpoints is a no-op when jsluice is not enabled or CGO is off.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	return nil
}
