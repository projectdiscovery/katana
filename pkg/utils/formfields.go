package utils

import (
	"github.com/projectdiscovery/katana/pkg/navigation"

	"github.com/PuerkitoBio/goquery"
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
		form.Method = method
		form.Enctype = enctype

		formElem.Find("input, textarea, select").Each(func(i int, inputElem *goquery.Selection) {
			name, ok := inputElem.Attr("name")
			if !ok {
				return
			}

			form.Parameters = append(form.Parameters, name)
		})

		forms = append(forms, form)
	})

	return forms
}
