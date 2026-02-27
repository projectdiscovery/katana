package utils

import (
	"regexp"
	"strings"

	"github.com/dop251/goja"
)

type JsluiceEndpoint struct {
	Endpoint string
	Type     string
}

var stringPattern = regexp.MustCompile(`(["'])(?:(?=(\\?))\2.)*?\1`)
var urlPattern = regexp.MustCompile(`(?i)^(?:/|[a-z0-9.\-_]+://|[a-z0-9_\-/]+\.(?:php|asp|aspx|jsp|json|js|html|xml))(?:[a-z0-9\-._~:/?#\[\]@!$&'()*+,;=]+)?$`)

func ExtractJsluiceEndpoints(data string) []JsluiceEndpoint {
	var endpoints []JsluiceEndpoint
	seen := make(map[string]bool)

	vm := goja.New()
	_, _ = vm.RunString(data)

	matches := stringPattern.FindAllStringSubmatch(data, -1)
	for _, match := range matches {
		if len(match) > 0 {
			val := strings.Trim(match[0], "\"'")
			val = strings.TrimSpace(val)

			if len(val) >= 2 && urlPattern.MatchString(val) {
				if !seen[val] {
					seen[val] = true
					endpoints = append(endpoints, JsluiceEndpoint{
						Endpoint: val,
						Type:     "stringLiteral",
					})
				}
			}
		}
	}

	return endpoints
}

// IsPathCommonJSLibraryFile checks if the path is a common js library file
func IsPathCommonJSLibraryFile(path string) bool {
	lowerPath := strings.ToLower(path)
	commonLibs := []string{
		"jquery", "bootstrap", "react", "vue", "angular", "lodash",
		"moment", "d3", "chart.js", "axios", "sweetalert", "toastr",
		"font-awesome", "modernizr", "popper.js", "slick", "swiper",
		"select2", "datatables", "core-js", "regenerator-runtime",
		"tslib", "rxjs", "zone.js", "reflect-metadata", "hammer.js",
		"webfontloader", "require.js", "require.min.js", "require",
		"mathjax", "highlight.js", "prism", "clipboard.js", "marked",
		"turndown", "dompurify", "xss", "crypto-js", "jsencrypt",
		"bcrypt", "uuid", "socket.io", "socket.io.js", "socket.io.min.js",
		"echarts", "highcharts", "amcharts", "plotly", "chartist",
		"three.js", "pixi.js", "phaser", "babylon.js", "aframe",
		"matter-js", "p5.js", "fabric.js", "konva", "paper.js",
		"tinymce", "ckeditor", "quill", "draft-js", "slate",
		"ace", "monaco-editor", "codemirror", "handsontable", "ag-grid",
		"fullcalendar", "dayjs", "date-fns", "luxon", "numeral",
		"accounting", "currency.js", "dinero.js", "mathjs", "decimal.js",
		"big.js", "bignumber.js", "fraction.js", "complex.js", "sylvester",
		"numeric", "gl-matrix", "gl-vec2", "gl-vec3", "gl-vec4",
		"gl-mat2", "gl-mat3", "gl-mat4", "gl-quat", "backbone", "underscore",
	}

	for _, lib := range commonLibs {
		if strings.Contains(lowerPath, lib) {
			return true
		}
	}
	return false
}

