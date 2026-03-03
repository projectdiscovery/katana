//go:build !jsluice || windows || 386

package utils

// ExtractJsluiceEndpoints returns no endpoints when jsluice support is not
// enabled for the current build.
func ExtractJsluiceEndpoints(data string) []JSLuiceEndpoint {
	return nil
}
