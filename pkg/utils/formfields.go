package utils

import (
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

		form.Action = action
		form.Method = strings.ToUpper(method)
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
