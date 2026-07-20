package browser

import (
	"context"
	"errors"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ClearSession wipes cookies and web storage so the next navigation starts clean.
func ClearSession(ctx context.Context, page *rod.Page) error {
	if page == nil {
		return nil
	}
	page = page.Context(ctx)

	var errs []error
	if err := (proto.NetworkClearBrowserCookies{}).Call(page); err != nil {
		errs = append(errs, err)
	}

	// Clear the DOM-accessible storage of the current origin.
	if _, err := page.Eval(`() => {
		try { localStorage.clear(); } catch (e) {}
		try { sessionStorage.clear(); } catch (e) {}
	}`); err != nil {
		errs = append(errs, err)
	}

	// localStorage.clear() misses IndexedDB, Cache Storage and service workers,
	// which SPAs use to persist auth. Clear all storage types for the current
	// origin via CDP so the rewalk really starts from a blank slate.
	if origin, err := page.Eval(`() => location.origin`); err == nil {
		if o := origin.Value.Str(); o != "" && o != "null" {
			if err := (proto.StorageClearDataForOrigin{
				Origin:       o,
				StorageTypes: "all",
			}).Call(page); err != nil {
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}
