package utils

import (
	"net/url"
	"strings"

	"github.com/projectdiscovery/katana/pkg/navigation"

	"github.com/PuerkitoBio/goquery"
	"github.com/projectdiscovery/utils/generic"
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

		actionUrl, _ := url.Parse(action)
		if !actionUrl.IsAbs() && !strings.HasPrefix(action, "//") && !strings.HasPrefix(action, "\\\\") {
			if action == "" {
				action = document.Url.String()
			} else if strings.HasPrefix(action, "/") {
				action, _ = url.JoinPath(document.Url.Scheme+"://"+document.Url.Host, action)
			} else if !strings.HasPrefix(action, "/") {
				action = document.Url.JoinPath(action).String()
			}
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
