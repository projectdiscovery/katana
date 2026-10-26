package browser

import (
	"context"
	"errors"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
)

// ClearSession wipes cookies and web storage of origins so the next navigation
// starts clean. Only the given origins are touched: the browser can be the
// user's own (-cwu, -cdd), and clearing it as a whole would log them out of
// every other site.
func ClearSession(ctx context.Context, page *rod.Page, origins []string) error {
	if page == nil || len(origins) == 0 {
		return nil
	}
	page = page.Context(ctx)

	var errs []error
	// Cookies of a parent domain (.example.com) apply to the origin too, so
	// delete what the browser would send to each origin, not only host cookies.
	cookies, err := proto.NetworkGetCookies{Urls: origins}.Call(page)
	if err != nil {
		errs = append(errs, err)
	} else {
		for _, cookie := range cookies.Cookies {
			if err := (proto.NetworkDeleteCookies{
				Name:   cookie.Name,
				Domain: cookie.Domain,
				Path:   cookie.Path,
			}).Call(page); err != nil {
				errs = append(errs, err)
			}
		}
	}

	// Local storage, IndexedDB, Cache Storage and service workers persist auth
	// in SPAs.
	for _, origin := range origins {
		if err := (proto.StorageClearDataForOrigin{
			Origin:       origin,
			StorageTypes: "all",
		}).Call(page); err != nil {
			errs = append(errs, err)
		}
	}

	// Session storage belongs to the tab, which the CDP call does not reach.
	if _, err := page.Eval(`() => { try { sessionStorage.clear(); } catch (e) {} }`); err != nil {
		errs = append(errs, err)
	}
	return errors.Join(errs...)
}
