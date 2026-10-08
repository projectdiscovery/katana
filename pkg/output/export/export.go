// Package export forwards observed HTTP traffic to an HTTP collector.
//
// Wrap returns an output.Writer that also implements Capture. Engines call
// Capture with every exchange they observe, before scope, dedupe and omit
// filters; exchanges are POSTed asynchronously as JSON batches so crawling
// never blocks on the collector. It is enabled only when
// KATANA_TRAFFIC_ENDPOINT and KATANA_TRAFFIC_TOKEN are set.
package export

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/projectdiscovery/katana/pkg/output"
)

const (
	EnvEndpoint      = "KATANA_TRAFFIC_ENDPOINT"
	EnvToken         = "KATANA_TRAFFIC_TOKEN"
	EnvAttributes    = "KATANA_TRAFFIC_ATTRIBUTES"
	EnvAllowInsecure = "KATANA_TRAFFIC_ALLOW_INSECURE"

	batchSize     = 100
	queueSize     = 2048
	maxBodyBytes  = 64 * 1024
	flushInterval = time.Second
	maxAttributes = 32
	maxAttrValue  = 1024
)

var (
	retryDelays = []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	attrKeyRule = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)
	// reservedKeys are event fields owned by the exporter.
	reservedKeys = map[string]struct{}{
		"id": {}, "timestamp": {}, "captured_at": {}, "source_type": {},
		"process": {}, "request": {}, "response": {},
	}
)

// Config controls an exporter. Endpoint and Token must come from trusted
// process configuration, never from crawled content.
type Config struct {
	Endpoint      string
	Token         string
	Attributes    map[string]string
	AllowInsecure bool
	HTTPClient    *http.Client
}

type writer struct {
	output.Writer
	config     Config
	attributes []byte
	events     chan json.RawMessage
	done       chan struct{}

	mu        sync.RWMutex
	closed    bool
	closeOnce sync.Once
	dropped   atomic.Int64
	err       error
}

type event struct {
	ID         string   `json:"id"`
	Timestamp  string   `json:"timestamp"`
	CapturedAt string   `json:"captured_at"`
	SourceType string   `json:"source_type"`
	Process    string   `json:"process"`
	Request    message  `json:"request"`
	Response   response `json:"response"`
}

type message struct {
	Method        string            `json:"method,omitempty"`
	FullURL       string            `json:"full_url,omitempty"`
	HTTPVersion   string            `json:"http_version"`
	Headers       map[string]string `json:"headers"`
	BodyPreview   string            `json:"body_preview,omitempty"`
	BodySHA256    string            `json:"body_sha256,omitempty"`
	BodySize      int64             `json:"body_size,omitempty"`
	BodyTruncated bool              `json:"body_truncated,omitempty"`
}

type response struct {
	Status      int    `json:"status"`
	StatusText  string `json:"status_text,omitempty"`
	ContentType string `json:"content_type,omitempty"`
	message
}

// Wrap returns inner unchanged when export is not configured. On a
// configuration error it also returns inner, so a bad setting never breaks
// crawling output.
func Wrap(inner output.Writer) (output.Writer, error) {
	endpoint := strings.TrimSpace(os.Getenv(EnvEndpoint))
	token := strings.TrimSpace(os.Getenv(EnvToken))
	if endpoint == "" && token == "" {
		return inner, nil
	}
	if endpoint == "" || token == "" {
		return inner, fmt.Errorf("both %s and %s are required", EnvEndpoint, EnvToken)
	}
	var attributes map[string]string
	if raw := strings.TrimSpace(os.Getenv(EnvAttributes)); raw != "" {
		if err := json.Unmarshal([]byte(raw), &attributes); err != nil {
			return inner, fmt.Errorf("%s must be a JSON object of string values", EnvAttributes)
		}
	}
	w, err := New(inner, Config{
		Endpoint:      endpoint,
		Token:         token,
		Attributes:    attributes,
		AllowInsecure: os.Getenv(EnvAllowInsecure) == "1",
	})
	if err != nil {
		return inner, err
	}
	return w, nil
}

// New wraps inner with an exporter for config.
func New(inner output.Writer, config Config) (output.Writer, error) {
	if err := validateEndpoint(config.Endpoint, config.AllowInsecure); err != nil {
		return nil, err
	}
	if config.Token == "" {
		return nil, errors.New("traffic export token is required")
	}
	attributes, err := encodeAttributes(config.Attributes)
	if err != nil {
		return nil, err
	}
	if config.HTTPClient == nil {
		transport := http.DefaultTransport.(*http.Transport).Clone()
		// Do not route the bearer token through an inherited HTTP_PROXY.
		transport.Proxy = nil
		config.HTTPClient = &http.Client{
			Transport: transport,
			Timeout:   15 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
	}
	w := &writer{
		Writer:     inner,
		config:     config,
		attributes: attributes,
		events:     make(chan json.RawMessage, queueSize),
		done:       make(chan struct{}),
	}
	go w.run()
	return w, nil
}

func validateEndpoint(endpoint string, allowInsecure bool) error {
	parsed, err := url.Parse(endpoint)
	if err != nil || parsed.Hostname() == "" {
		return errors.New("invalid traffic export endpoint")
	}
	if parsed.User != nil || parsed.RawQuery != "" || parsed.Fragment != "" {
		return errors.New("traffic export endpoint cannot contain credentials, query, or fragment")
	}
	switch parsed.Scheme {
	case "https":
		return nil
	case "http":
		if allowInsecure || isLoopback(parsed.Hostname()) {
			return nil
		}
		return errors.New("plain http traffic export is allowed only for loopback unless explicitly enabled")
	default:
		return errors.New("traffic export endpoint must use http or https")
	}
}

func isLoopback(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// encodeAttributes validates attributes and returns them as JSON object
// members (without braces) to append to every event.
func encodeAttributes(attributes map[string]string) ([]byte, error) {
	if len(attributes) == 0 {
		return nil, nil
	}
	if len(attributes) > maxAttributes {
		return nil, fmt.Errorf("at most %d traffic export attributes are allowed", maxAttributes)
	}
	for key, value := range attributes {
		if _, reserved := reservedKeys[key]; reserved || !attrKeyRule.MatchString(key) {
			return nil, fmt.Errorf("invalid traffic export attribute %q", key)
		}
		if len(value) > maxAttrValue {
			return nil, fmt.Errorf("traffic export attribute %q exceeds %d bytes", key, maxAttrValue)
		}
	}
	encoded, err := json.Marshal(attributes)
	if err != nil {
		return nil, err
	}
	return encoded[1 : len(encoded)-1], nil
}

// Capture queues an observed exchange for export. It never blocks: when the
// queue is full the exchange is dropped and counted.
func (w *writer) Capture(result *output.Result) {
	encoded, ok := w.encode(result)
	if !ok {
		return
	}
	w.mu.RLock()
	defer w.mu.RUnlock()
	if w.closed {
		w.dropped.Add(1)
		return
	}
	select {
	case w.events <- encoded:
	default:
		w.dropped.Add(1)
	}
}

func (w *writer) Close() error {
	innerErr := w.Writer.Close()
	w.closeOnce.Do(func() {
		w.mu.Lock()
		w.closed = true
		close(w.events)
		w.mu.Unlock()
		<-w.done
		if w.err == nil {
			if dropped := w.dropped.Load(); dropped > 0 {
				w.err = fmt.Errorf("traffic export incomplete: %d events dropped", dropped)
			}
		}
	})
	if innerErr != nil {
		return innerErr
	}
	return w.err
}

func (w *writer) run() {
	defer close(w.done)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	batch := make([]json.RawMessage, 0, batchSize)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		// After a failed batch, stop sending and count the rest as dropped.
		if w.err != nil {
			w.dropped.Add(int64(len(batch)))
		} else if err := w.post(batch); err != nil {
			w.dropped.Add(int64(len(batch)))
			w.err = err
		}
		batch = batch[:0]
	}
	for {
		select {
		case ev, ok := <-w.events:
			if !ok {
				flush()
				return
			}
			batch = append(batch, ev)
			if len(batch) >= batchSize {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// encode serialises a result synchronously, since callers mutate it (omit
// flags) after Capture returns. Results without a real HTTP response,
// such as errors or URLs discovered but not fetched, are skipped.
func (w *writer) encode(result *output.Result) (json.RawMessage, bool) {
	if result == nil || result.Request == nil || !result.HasResponse() {
		return nil, false
	}
	req, resp := result.Request, result.Response
	timestamp := result.Timestamp
	if timestamp.IsZero() {
		timestamp = time.Now()
	}
	contentType := resp.Resp.Header.Get("Content-Type")
	responseBody := resp.Body
	if !isTextual(contentType) || !utf8.ValidString(responseBody) {
		// JSON cannot carry these bytes verbatim, so a preview would not
		// match body_sha256.
		responseBody = ""
	}
	requestVersion := "HTTP/1.1"
	if resp.Resp.Request != nil && resp.Resp.Request.Proto != "" {
		requestVersion = resp.Resp.Request.Proto
	}
	responseVersion := valueOr(resp.Resp.Proto, "HTTP/1.1")
	ev := event{
		ID:         uuid.NewString(),
		Timestamp:  timestamp.UTC().Format(time.RFC3339Nano),
		CapturedAt: time.Now().UTC().Format(time.RFC3339Nano),
		SourceType: "cli",
		Process:    "katana",
		Request:    newMessage(req.Method, req.URL, requestVersion, req.Headers, req.Body),
		Response: response{
			Status:      resp.StatusCode,
			StatusText:  http.StatusText(resp.StatusCode),
			ContentType: contentType,
			message:     newMessage("", "", responseVersion, resp.Headers, responseBody),
		},
	}
	if responseBody == "" && resp.Body != "" {
		// Binary or non-UTF-8 body: report its size and hash without a preview.
		ev.Response.BodySHA256 = sha256Hex(resp.Body)
		ev.Response.BodySize = int64(len(resp.Body))
		ev.Response.BodyTruncated = true
	}
	if resp.ContentLength > ev.Response.BodySize {
		ev.Response.BodySize = resp.ContentLength
		ev.Response.BodyTruncated = true
	}
	encoded, err := json.Marshal(ev)
	if err != nil {
		return nil, false
	}
	if len(w.attributes) > 0 {
		encoded = append(append(append(encoded[:len(encoded)-1], ','), w.attributes...), '}')
	}
	return encoded, true
}

func newMessage(method, fullURL, version string, headers map[string]string, body string) message {
	m := message{
		Method:      method,
		FullURL:     fullURL,
		HTTPVersion: version,
		Headers:     make(map[string]string, len(headers)),
		BodySHA256:  sha256Hex(body),
		BodySize:    int64(len(body)),
	}
	for key, value := range headers {
		m.Headers[key] = value
	}
	switch {
	case !utf8.ValidString(body):
		m.BodyTruncated = true
	case len(body) > maxBodyBytes:
		// Cut on a rune boundary so the preview stays valid UTF-8.
		m.BodyPreview = strings.ToValidUTF8(body[:maxBodyBytes], "")
		m.BodyTruncated = true
	default:
		m.BodyPreview = body
	}
	return m
}

func (w *writer) post(events []json.RawMessage) error {
	payload, err := json.Marshal(struct {
		BatchID string            `json:"batch_id"`
		Events  []json.RawMessage `json:"events"`
	}{uuid.NewString(), events})
	if err != nil {
		return err
	}
	for attempt := 0; ; attempt++ {
		retryable, err := w.postOnce(payload)
		if err == nil {
			return nil
		}
		if !retryable || attempt >= len(retryDelays) {
			return err
		}
		time.Sleep(retryDelays[attempt])
	}
}

func (w *writer) postOnce(payload []byte) (retryable bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, w.config.Endpoint, bytes.NewReader(payload))
	if err != nil {
		return false, errors.New("could not create traffic export request")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+w.config.Token)
	resp, err := w.config.HTTPClient.Do(req)
	if err != nil {
		return true, errors.New("traffic export request failed")
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retryable := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return retryable, fmt.Errorf("traffic collector rejected batch with status %d", resp.StatusCode)
	}
	return false, nil
}

func sha256Hex(value string) string {
	if value == "" {
		return ""
	}
	digest := sha256.Sum256([]byte(value))
	return hex.EncodeToString(digest[:])
}

func isTextual(contentType string) bool {
	if contentType == "" {
		return true
	}
	lower := strings.ToLower(contentType)
	for _, marker := range []string{"text/", "json", "xml", "javascript", "graphql", "x-www-form-urlencoded"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func valueOr(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
