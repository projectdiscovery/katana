package utils

import (
	"context"
	"errors"
	"testing"

	"github.com/projectdiscovery/katana/pkg/decider"
	mapsutil "github.com/projectdiscovery/utils/maps"
	"github.com/stretchr/testify/require"
)

func formInput(typ, name string, attrs ...string) FormInput {
	input := FormInput{Type: typ, Name: name, Attributes: mapsutil.NewOrderedMap[string, string]()}
	for i := 0; i+1 < len(attrs); i += 2 {
		input.Attributes.Set(attrs[i], attrs[i+1])
	}
	return input
}

func TestSemanticValue(t *testing.T) {
	FormData = DefaultFormFillData
	tests := []struct {
		name  string
		input FormInput
		want  string
	}{
		{"html type", formInput("URL", "homepage_link"), "https://example.com"},
		{"autocomplete wins over name", formInput("text", "field1", "autocomplete", "shipping postal-code"), "10001"},
		{"camel case name", formInput("text", "billingFirstName"), "John"},
		{"snake case name", formInput("text", "zip_code"), "10001"},
		{"id when name is opaque", formInput("text", "f3", "id", "user-city"), "New York"},
		{"compound before shorter word", formInput("text", "user_name"), "katana"},
		{"word inside another word", formInput("text", "nickname"), ""},
		{"no signal", formInput("text", "q"), ""},
		{"pattern kept when it matches", formInput("text", "zip", "pattern", "[0-9]{5}"), "10001"},
		{"pattern generates when value fails", formInput("tel", "phone", "pattern", `\d{3}-\d{4}`), "918-9298"},
		{"pattern without a semantic value", formInput("text", "code", "pattern", "[A-Z]{3}"), "ZCW"},
		{"pattern re2 cannot parse", formInput("text", "zip", "pattern", "(?=x)"), "10001"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, semanticValue(tt.input))
		})
	}
}

type formStub struct {
	answer decider.Answer
	err    error
	asked  []decider.Question
}

func (s *formStub) Decide(_ context.Context, q decider.Question) (decider.Answer, error) {
	s.asked = append(s.asked, q)
	return s.answer, s.err
}

func TestSemanticValueAsksDecider(t *testing.T) {
	FormData = DefaultFormFillData
	defer func() { FormDecider = nil }()

	stub := &formStub{answer: decider.Answer{Choice: "zip"}}
	FormDecider = stub
	require.Equal(t, "10001", semanticValue(formInput("text", "plz", "placeholder", "Postleitzahl")))
	require.Len(t, stub.asked, 1)
	require.Equal(t, `<input type="text" name="plz" placeholder="Postleitzahl">`, stub.asked[0].Context)
	require.Contains(t, stub.asked[0].Options, "other")

	require.Equal(t, "John", semanticValue(formInput("text", "first_name")))
	require.Len(t, stub.asked, 1, "rules answer first")

	require.Empty(t, semanticValue(formInput("submit", "go")))
	require.Empty(t, semanticValue(formInput("hidden", "token")))
	require.Len(t, stub.asked, 1, "only text fields are asked about")

	FormDecider = &formStub{answer: decider.Answer{Choice: "other"}}
	require.Empty(t, semanticValue(formInput("text", "plz")))

	FormDecider = &formStub{err: errors.New("down")}
	require.Empty(t, semanticValue(formInput("text", "plz")))
}
