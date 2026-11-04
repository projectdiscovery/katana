package hybrid

import (
	"encoding/base64"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// NewHijack create hijack from page.
func NewHijack(page *rod.Page) *Hijack {
	return &Hijack{
		page:    page,
		disable: &proto.FetchDisable{},
	}
}

// HijackHandler type
type HijackHandler = func(e *proto.FetchRequestPaused) error

// Hijack is a hijack handler
type Hijack struct {
	page    *rod.Page
	p       *rod.Page
	enable  *proto.FetchEnable
	disable *proto.FetchDisable
	cancel  func()
}

// Page returns the scoped page instance with cancellation context if started,
// or the fallback page.
func (h *Hijack) Page() *rod.Page {
	if h.p != nil {
		return h.p
	}
	return h.page
}

// SetPattern set pattern directly
func (h *Hijack) SetPattern(pattern *proto.FetchRequestPattern) {
	h.enable = &proto.FetchEnable{
		Patterns: []*proto.FetchRequestPattern{pattern},
	}
}

// Start hijack.
func (h *Hijack) Start(handler HijackHandler) func() error {
	if h.enable == nil {
		panic("hijack pattern not set")
	}

	p, cancel := h.page.WithCancel()
	h.p = p
	h.cancel = cancel

	err := h.enable.Call(p)
	if err != nil {
		return func() error { return err }
	}

	wait := p.EachEvent(func(e *proto.FetchRequestPaused) {
		if handler != nil {
			err = handler(e)
		}
	})

	return func() error {
		wait()
		return err
	}
}

// Stop
func (h *Hijack) Stop() error {
	if h.cancel != nil {
		h.cancel()
	}
	return h.disable.Call(h.page)
}

// FetchGetResponseBody get request body.
func FetchGetResponseBody(page *rod.Page, e *proto.FetchRequestPaused) ([]byte, error) {
	m := proto.FetchGetResponseBody{
		RequestID: e.RequestID,
	}
	r, err := m.Call(page)
	if err != nil {
		return nil, err
	}

	if !r.Base64Encoded {
		return []byte(r.Body), nil
	}

	bs, err := base64.StdEncoding.DecodeString(r.Body)
	if err != nil {
		return nil, err
	}
	return bs, nil
}

// FetchContinueRequest continue request
func FetchContinueRequest(page *rod.Page, e *proto.FetchRequestPaused) error {
	m := proto.FetchContinueRequest{
		RequestID: e.RequestID,
	}
	return m.Call(page)
}
