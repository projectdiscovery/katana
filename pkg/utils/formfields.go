package utils

import (
	"strings"

	"github.com/projectdiscovery/katana/pkg/navigation"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/utils/generic"
	urlutil "github.com/projectdiscovery/utils/url"
)

// parses form, input, textarea & select elements
func ParseFormFields(document *goquery.Document) []navigation.Form {
	var forms []navigation.Form

	document.Find("form").Each(func(i int, formElem *goquery.Selection) {
		form := navigation.Form{}

		action, _ := formElem.Attr("action")
		method, _ := formElem.Attr("method")
		enctype, _ := formElem.Attr("enctype")

		if method == "" {
			method = "GET"
		}

		if enctype == "" && method != "GET" {
			enctype = "application/x-www-form-urlencoded"
		}

		if action != "" {
			actionUrl, err := urlutil.ParseURL(action, true)
			if err != nil {
				return
			}

			// donot modify absolute urls and windows paths
			if actionUrl.IsAbs() || strings.HasPrefix(action, "//") || strings.HasPrefix(action, "\\") {
				// keep absolute urls as is
				_ = action
			} else if document.Url != nil {
				// concatenate relative urls with base url
				// clone base url
				cloned, err := urlutil.ParseURL(document.Url.String(), true)
				if err != nil {
					return
				}

				if strings.HasPrefix(action, "/") {
					// relative path
					// 	<form action=/root_rel></form> => https://example.com/root_rel
					_ = cloned.UpdateRelPath(action, true)
					action = cloned.String()
				} else {
					// 	<form action=path_rel></form> => https://example.com/path/path_rel
					if err := cloned.MergePath(action, false); err != nil {
						return
					}
					action = cloned.String()
				}
			}
		} else {
			action = document.Url.String()
		}

		form.Method = strings.ToUpper(method)
		form.Action = action
		form.Enctype = enctype

		formElem.Find("input, textarea, select").Each(func(i int, inputElem *goquery.Selection) {
			name, ok := inputElem.Attr("name")
			if !ok {
				return
			}

			form.Parameters = append(form.Parameters, name)
		})

		if !generic.EqualsAll("", form.Action, form.Method, form.Enctype) || len(form.Parameters) > 0 {
			forms = append(forms, form)
		}
	})

	return forms
}
