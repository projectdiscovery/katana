//go:build !(386 || windows)

package utils

import (
	"fmt"
	"runtime"
	"strings"
	"testing"
)

func TestIsPathCommonJSLibraryFile(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			name: "jquery.js",
			args: args{
				path: "jquery.js",
			},
			want: true,
		},
		{
			name: "app.js",
			args: args{
				path: "app.js",
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsPathCommonJSLibraryFile(tt.args.path); got != tt.want {
				t.Errorf("IsPathCommonJSLibraryFile() = %v, want %v", got, tt.want)
			}
		})
	}
}

const escapedQuotesURL = "https://example.com/escaped-quotes"

// escapedQuotesJS mimics a Next.js flight payload: one string literal with many \" sequences.
// The fetch call is a separate literal so extraction still has a URL to return.
func escapedQuotesJS(n int) string {
	var b strings.Builder
	b.WriteString(`x("`)
	for i := range n {
		fmt.Fprintf(&b, `\"k%d\":\"v\",`, i)
	}
	b.WriteString("\");\n")
	fmt.Fprintf(&b, "fetch(%q);\n", escapedQuotesURL)
	return b.String()
}

func TestExtractJsluiceEndpointsEscapedQuotesMemory(t *testing.T) {
	const maxAlloc = 64 << 20
	for _, n := range []int{250, 500, 1000} {
		t.Run(fmt.Sprint(n), func(t *testing.T) {
			data := escapedQuotesJS(n)
			var before, after runtime.MemStats
			runtime.GC()
			runtime.ReadMemStats(&before)
			endpoints := ExtractJsluiceEndpoints(data)
			runtime.ReadMemStats(&after)
			foundURL := false
			for _, endpoint := range endpoints {
				if endpoint.Endpoint == escapedQuotesURL {
					foundURL = true
					break
				}
			}
			if !foundURL {
				t.Fatalf("extracted endpoints = %#v, want %s", endpoints, escapedQuotesURL)
			}
			if alloc := after.TotalAlloc - before.TotalAlloc; alloc > maxAlloc {
				t.Errorf("allocated %d MiB for %d byte input, want <= %d MiB", alloc>>20, len(data), maxAlloc>>20)
			}
		})
	}
}
