package browser

import (
	"context"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ClearSession wipes cookies and web storage so the next navigation starts clean.
func ClearSession(ctx context.Context, page *rod.Page) error {
	if page == nil {
		return nil
	}
	page = page.Context(ctx)
	_ = proto.NetworkClearBrowserCookies{}.Call(page)
	_, err := page.Eval(`() => {
		try { localStorage.clear(); } catch (e) {}
		try { sessionStorage.clear(); } catch (e) {}
	}`)
	return err
}
