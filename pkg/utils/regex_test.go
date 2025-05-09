package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPageBodyRegex(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "Mix of patterns",
			input: `Some text <a href="./rel/file.txt">link1</a> and <img src="../rel2/file.php"/> also http://a.com/b.html and https://c.com/d.aspx?p=1 finally /abs/path.js and rel/path/script.py end`,
			expected: []string{
				"./rel/file.txt",           // BodyC0
				"../rel2/file.php",         // BodyC0
				"http://a.com/b.html",      // BodyC1
				"https://c.com/d.aspx?p=1", // BodyC1
				"/abs/path.js",             // BodyC2
				"rel/path/script.py",       // BodyC3
			},
		},
		{
			name:     "No matches",
			input:    "Just some plain text without any URLs or paths.",
			expected: []string{},
		},
		{
			name:     "Only BodyC0",
			input:    `"./path1" '../path2'`,
			expected: []string{"./path1", "../path2"},
		},
		{
			name:     "Only BodyC1",
			input:    `http://example.com/page1 https://secure.com/page2`,
			expected: []string{"http://example.com/page1", "https://secure.com/page2"},
		},
		{
			name:     "Only BodyC2",
			input:    `"/path/to/file.css" '/another/script.js'`,
			expected: []string{"/path/to/file.css", "/another/script.js"},
		},
		{
			name:     "Only BodyC3",
			input:    `"relative/path/file.php" 'another/relative/page.html'`,
			expected: []string{"relative/path/file.php", "another/relative/page.html"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := pageBodyRegex.FindAllStringSubmatch(tc.input, -1)
			actual := []string{}
			for _, match := range matches {
				if len(match) > 1 {
					actual = append(actual, match[1]) // Extract the first capture group
				}
			}
			assert.ElementsMatch(t, tc.expected, actual, "Input: %s", tc.input)
		})
	}
}

func TestRelativeEndpointsRegex(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected []string
	}{
		{
			name:  "Mix of patterns in JS-like context",
			input: `var u1 = "https://d.com/e.php?q=1"; let u2 = './f/g.js'; const u3 = '../h/i.html'; func('/j/k/lll'); load('m/nnn/'); action("o/p.action");`,
			expected: []string{
				"https://d.com/e.php?q=1", // JsC0
				"./f/g.js",                // JsC1
				"../h/i.html",             // JsC1
				"/j/k/lll",                // JsC2
				"m/nnn/",                  // JsC3
				"o/p.action",              // JsC1
			},
		},
		{
			name:     "No matches",
			input:    "var x = 1; let y = 'hello'; const z = true;",
			expected: []string{},
		},
		{
			name:     "Only JsC0",
			input:    `"https://example.com/api/v1?key=123" 'http://localhost:8080/test#section'`,
			expected: []string{"https://example.com/api/v1?key=123", "http://localhost:8080/test#section"},
		},
		{
			name:     "Only JsC1",
			input:    `"./script.js" 'page.php?id=5'`,
			expected: []string{"./script.js", "page.php?id=5"},
		},
		{
			name:     "Only JsC2",
			input:    `"/api/v2/users" '/data/items/fetch'`,
			expected: []string{"/api/v2/users", "/data/items/fetch"},
		},
		{
			name:     "Only JsC3",
			input:    `"./images/" '../assets/' "static/"`,
			expected: []string{"./images/", "../assets/", "static/"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			matches := relativeEndpointsRegex.FindAllStringSubmatch(tc.input, -1)
			actual := []string{}
			for _, match := range matches {
				if len(match) > 1 {
					actual = append(actual, match[1]) // Extract the first capture group
				}
			}
			assert.ElementsMatch(t, tc.expected, actual, "Input: %s", tc.input)
		})
	}
}
