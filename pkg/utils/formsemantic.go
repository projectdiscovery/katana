package utils

import (
	"context"
	"fmt"
	"maps"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/brianvoe/gofakeit/v7"
	"github.com/projectdiscovery/katana/pkg/decider"
)

// FormDecider classifies fields the rules below cannot. Nil disables it.
var FormDecider decider.Decider

// fixedValues are constant so repeated crawls submit the same forms.
var fixedValues = map[string]string{
	"url":        "https://example.com",
	"date":       "2024-01-15",
	"time":       "12:00",
	"datetime":   "2024-01-15T12:00",
	"month":      "2024-01",
	"week":       "2024-W03",
	"zip":        "10001",
	"username":   "katana",
	"name":       "John Doe",
	"firstname":  "John",
	"lastname":   "Doe",
	"address":    "1 Main St",
	"city":       "New York",
	"state":      "NY",
	"country":    "US",
	"company":    "Example Inc",
	"age":        "30",
	"amount":     "1",
	"cardnumber": "4111111111111111",
	"cardexpiry": "12/30",
	"cvv":        "123",
	"ip":         "127.0.0.1",
}

// semanticTypes maps html input types to semantic types.
var semanticTypes = map[string]string{
	"email":          "email",
	"tel":            "phone",
	"password":       "password",
	"url":            "url",
	"date":           "date",
	"time":           "time",
	"datetime-local": "datetime",
	"month":          "month",
	"week":           "week",
}

// autocompleteTypes maps html autocomplete tokens to semantic types.
var autocompleteTypes = map[string]string{
	"email":            "email",
	"tel":              "phone",
	"tel-national":     "phone",
	"current-password": "password",
	"new-password":     "password",
	"url":              "url",
	"bday":             "date",
	"postal-code":      "zip",
	"username":         "username",
	"name":             "name",
	"given-name":       "firstname",
	"family-name":      "lastname",
	"street-address":   "address",
	"address-line1":    "address",
	"address-level2":   "city",
	"address-level1":   "state",
	"country":          "country",
	"country-name":     "country",
	"organization":     "company",
	"cc-number":        "cardnumber",
	"cc-exp":           "cardexpiry",
	"cc-csc":           "cvv",
}

// fieldKeywords is ordered: compound names must match before the
// shorter words they contain, such as username before name.
var fieldKeywords = []struct {
	semantic string
	words    []string
}{
	{"email", []string{"email", "mail"}},
	{"password", []string{"password", "passwd", "pass", "pwd"}},
	{"username", []string{"username", "uname", "login", "userid"}},
	{"firstname", []string{"firstname", "fname", "givenname", "forename"}},
	{"lastname", []string{"lastname", "lname", "surname", "familyname"}},
	{"phone", []string{"phone", "telephone", "tel", "mobile", "cell"}},
	{"zip", []string{"zip", "zipcode", "postcode", "postal", "postalcode"}},
	{"date", []string{"date", "dob", "birthday", "birthdate"}},
	{"url", []string{"url", "uri", "website", "homepage"}},
	{"cardnumber", []string{"cardnumber", "ccnumber", "creditcard"}},
	{"cvv", []string{"cvv", "cvc", "csc"}},
	{"ip", []string{"ip", "ipaddress"}},
	{"address", []string{"address", "street", "addr"}},
	{"city", []string{"city", "town"}},
	{"state", []string{"state", "province", "region"}},
	{"country", []string{"country"}},
	{"company", []string{"company", "organization", "organisation"}},
	{"age", []string{"age"}},
	{"amount", []string{"amount", "price", "quantity", "qty"}},
	{"name", []string{"name", "fullname"}},
}

// semanticValue picks a value for an empty input, or "" to keep the placeholder.
func semanticValue(input FormInput) string {
	value := typedValue(fieldSemantic(input))
	if pattern, ok := input.Attributes.Get("pattern"); ok {
		value = matchPattern(pattern, value)
	}
	return value
}

func fieldSemantic(input FormInput) string {
	if semantic, ok := semanticTypes[strings.ToLower(input.Type)]; ok {
		return semantic
	}
	if autocomplete, ok := input.Attributes.Get("autocomplete"); ok {
		tokens := strings.Fields(strings.ToLower(autocomplete))
		if len(tokens) > 0 {
			if semantic, ok := autocompleteTypes[tokens[len(tokens)-1]]; ok {
				return semantic
			}
		}
	}
	id, _ := input.Attributes.Get("id")
	words := fieldWords(input.Name + " " + id)
	for _, keyword := range fieldKeywords {
		if slices.ContainsFunc(keyword.words, func(w string) bool { return words[w] }) {
			return keyword.semantic
		}
	}
	return askDecider(input)
}

// fieldWords splits names like user_firstName into words and adjacent
// word pairs, so firstName matches firstname but nickname never matches name.
func fieldWords(s string) map[string]bool {
	var parts []string
	var current []rune
	flush := func() {
		if len(current) > 0 {
			parts = append(parts, strings.ToLower(string(current)))
			current = current[:0]
		}
	}
	for i, r := range s {
		switch {
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			flush()
		case unicode.IsUpper(r) && i > 0 && len(current) > 0 && unicode.IsLower(current[len(current)-1]):
			flush()
			current = append(current, r)
		default:
			current = append(current, r)
		}
	}
	flush()

	words := make(map[string]bool, 2*len(parts))
	for i, part := range parts {
		words[part] = true
		if i > 0 {
			words[parts[i-1]+part] = true
		}
	}
	return words
}

func typedValue(semantic string) string {
	switch semantic {
	case "email":
		return FormData.Email
	case "phone":
		return FormData.PhoneNumber
	case "password":
		return FormData.Password
	}
	return fixedValues[semantic]
}

// matchPattern keeps value when it satisfies the html pattern attribute and
// otherwise generates one that does. Patterns RE2 cannot parse are ignored.
func matchPattern(pattern, value string) string {
	anchored := "^(?:" + pattern + ")$"
	re, err := regexp.Compile(anchored)
	if err != nil || (value != "" && re.MatchString(value)) {
		return value
	}
	// a fixed seed keeps the generated value stable across runs
	generated := gofakeit.New(1).Regex(anchored)
	if !re.MatchString(generated) {
		return value
	}
	return generated
}

func askDecider(input FormInput) string {
	if FormDecider == nil {
		return ""
	}
	options := append(slices.Sorted(maps.Keys(fixedValues)), "email", "phone", "password", "other")
	answer, err := FormDecider.Decide(context.Background(), decider.Question{
		Kind:    decider.Choice,
		Prompt:  "Which kind of value does this form field expect? Answer other if none fit.",
		Options: options,
		Context: describeInput(input),
	})
	if err != nil || answer.Choice == "other" {
		return ""
	}
	return answer.Choice
}

// describeInput renders the field as html, leaving out the value attribute.
func describeInput(input FormInput) string {
	var b strings.Builder
	_, _ = fmt.Fprintf(&b, "<input type=%q name=%q", input.Type, input.Name)
	input.Attributes.Iterate(func(key, value string) bool {
		_, _ = fmt.Fprintf(&b, " %s=%q", key, value)
		return true
	})
	b.WriteString(">")
	return b.String()
}
