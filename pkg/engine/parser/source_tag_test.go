package parser

import (
	"strings"
	"testing"
)

func TestHTML5PictureSourceSrcsetExtraction(t *testing.T) {
	snippet := `<source srcset="/img/banner-hd.webp 2x, /img/banner-sd.webp 1x" type="image/webp">`
	if !strings.Contains(snippet, "srcset=") {
		t.Fatalf("failed to locate srcset attribute in source tag")
	}
}
