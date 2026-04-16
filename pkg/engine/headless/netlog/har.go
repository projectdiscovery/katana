package netlog

import (
	"encoding/json"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/projectdiscovery/katana/pkg/output"
)

// HAR 1.2 spec structs — minimal subset needed for request/response capture.

type HAR struct {
	Log HARLog `json:"log"`
}

type HARLog struct {
	Version string     `json:"version"`
	Creator HARCreator `json:"creator"`
	Entries []HAREntry `json:"entries"`
}

type HARCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type HAREntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            float64     `json:"time"`
	Request         HARRequest  `json:"request"`
	Response        HARResponse `json:"response"`
	Cache           struct{}    `json:"cache"`
	Timings         HARTimings  `json:"timings"`
	Comment         string      `json:"comment,omitempty"`
}

type HARRequest struct {
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	HTTPVersion string         `json:"httpVersion"`
	Headers     []HARNameValue `json:"headers"`
	QueryString []HARNameValue `json:"queryString"`
	PostData    *HARPostData   `json:"postData,omitempty"`
	HeadersSize int            `json:"headersSize"`
	BodySize    int            `json:"bodySize"`
}

type HARResponse struct {
	Status      int            `json:"status"`
	StatusText  string         `json:"statusText"`
	HTTPVersion string         `json:"httpVersion"`
	Headers     []HARNameValue `json:"headers"`
	Content     HARContent     `json:"content"`
	RedirectURL string         `json:"redirectURL"`
	HeadersSize int            `json:"headersSize"`
	BodySize    int            `json:"bodySize"`
}

type HARContent struct {
	Size     int    `json:"size"`
	MimeType string `json:"mimeType"`
	Text     string `json:"text,omitempty"`
}

type HARPostData struct {
	MimeType string `json:"mimeType"`
	Text     string `json:"text"`
}

type HARNameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type HARTimings struct {
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
}

// staticAssetExts are file extensions to filter from HAR output (except .js).
var staticAssetExts = map[string]bool{
	".css": true, ".woff": true, ".woff2": true, ".ttf": true, ".eot": true,
	".ico": true, ".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".svg": true, ".webp": true, ".map": true, ".avif": true,
}

func isFilteredAsset(rawURL string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	p := strings.ToLower(u.Path)
	// Check extension
	for ext := range staticAssetExts {
		if strings.HasSuffix(p, ext) {
			return true
		}
	}
	return false
}

// HARWriter collects HTTP entries and writes a HAR file on Close.
type HARWriter struct {
	mu      sync.Mutex
	entries []HAREntry
	path    string
}

// NewHARWriter creates a HAR writer that will flush to the given path.
func NewHARWriter(path string) *HARWriter {
	return &HARWriter{path: path}
}

// AddEntry converts an output.Result to a HAR entry and buffers it.
// Filters out static assets (CSS, fonts, images) but keeps JS files.
func (w *HARWriter) AddEntry(rr *output.Result) {
	if rr == nil || rr.Request == nil {
		return
	}
	// Filter static assets (keep JS)
	if isFilteredAsset(rr.Request.URL) {
		return
	}

	entry := HAREntry{
		StartedDateTime: rr.Timestamp.Format(time.RFC3339Nano),
		Timings:         HARTimings{Send: -1, Wait: -1, Receive: -1},
	}

	// Request
	entry.Request = HARRequest{
		Method:      rr.Request.Method,
		URL:         rr.Request.URL,
		HTTPVersion: "HTTP/1.1",
		Headers:     headersToNV(rr.Request.Headers),
		HeadersSize: -1,
		BodySize:    len(rr.Request.Body),
	}

	// Query string
	if u, err := url.Parse(rr.Request.URL); err == nil {
		for k, vals := range u.Query() {
			for _, v := range vals {
				entry.Request.QueryString = append(entry.Request.QueryString, HARNameValue{Name: k, Value: v})
			}
		}
	}

	// POST body
	if rr.Request.Body != "" {
		ct := rr.Request.Headers["content-type"]
		if ct == "" {
			ct = "application/x-www-form-urlencoded"
		}
		entry.Request.PostData = &HARPostData{
			MimeType: ct,
			Text:     rr.Request.Body,
		}
	}

	// Response
	if rr.Response != nil {
		ct := rr.Response.Headers["content-type"]
		if ct == "" {
			ct = "text/html"
		}
		body := rr.Response.Body
		entry.Response = HARResponse{
			Status:      rr.Response.StatusCode,
			StatusText:  statusText(rr.Response.StatusCode),
			HTTPVersion: "HTTP/1.1",
			Headers:     headersToNV(rr.Response.Headers),
			Content: HARContent{
				Size:     len(body),
				MimeType: ct,
				Text:     body,
			},
			HeadersSize: -1,
			BodySize:    len(body),
		}
	}

	w.mu.Lock()
	w.entries = append(w.entries, entry)
	w.mu.Unlock()
}

// Close writes the HAR file to disk.
func (w *HARWriter) Close() error {
	w.mu.Lock()
	entries := w.entries
	w.mu.Unlock()

	har := HAR{
		Log: HARLog{
			Version: "1.2",
			Creator: HARCreator{Name: "katana", Version: "1.0"},
			Entries: entries,
		},
	}

	data, err := json.MarshalIndent(har, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(w.path, data, 0644)
}

func headersToNV(h map[string]string) []HARNameValue {
	if h == nil {
		return []HARNameValue{}
	}
	nv := make([]HARNameValue, 0, len(h))
	for k, v := range h {
		nv = append(nv, HARNameValue{Name: k, Value: v})
	}
	return nv
}

func statusText(code int) string {
	texts := map[int]string{
		200: "OK", 201: "Created", 204: "No Content",
		301: "Moved Permanently", 302: "Found", 304: "Not Modified",
		400: "Bad Request", 401: "Unauthorized", 403: "Forbidden",
		404: "Not Found", 405: "Method Not Allowed", 500: "Internal Server Error",
	}
	if t, ok := texts[code]; ok {
		return t
	}
	return ""
}
