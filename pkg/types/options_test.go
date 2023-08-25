package types

import (
	"strings"
	"testing"

	"github.com/projectdiscovery/goflags"
	"github.com/stretchr/testify/require"
)

func TestParseCustomHeaders(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "single value",
			input: "a:b",
			want:  map[string]string{"a": "b"},
		},
		{
			name:  "empty string",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "empty value",
			input: "a:",
			want:  map[string]string{"a": ""},
		},
		{
			name:  "double input",
			input: "a:b,c:d",
			want:  map[string]string{"a": "b", "c": "d"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strsl := goflags.StringSlice{}
			for _, v := range strings.Split(tt.input, ",") {
				//nolint
				strsl.Set(v)
			}
			opt := Options{CustomHeaders: strsl}
			got := opt.ParseCustomHeaders()
			require.Equal(t, tt.want, got)
		})
	}
}

func TestParseHeadlessOptionalArguments(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  map[string]string
	}{
		{
			name:  "single value",
			input: "a=b",
			want:  map[string]string{"a": "b"},
		},
		{
			name:  "empty string",
			input: "",
			want:  map[string]string{},
		},
		{
			name:  "empty key",
			input: "=b",
			want:  map[string]string{},
		},
		{
			name:  "empty value",
			input: "a=",
			want:  map[string]string{},
		},
		{
			name:  "double input",
			input: "a=b,c=d",
			want:  map[string]string{"a": "b", "c": "d"},
		},
		{
			name:  "duplicated input",
			input: "a=b,a=b",
			want:  map[string]string{"a": "b"},
		},
		{
			name:  "values with dash with boolean flag at the end",
			input: "--a=a/b,c/d--z--n--m/a,--c=k,--h",
			want:  map[string]string{"--a": "a/b,c/d--z--n--m/a", "--c": "k", "--h": ""},
		},
		{
			name:  "values with dash boolean flag at the beginning",
			input: "--h,--a=a/b,c/d--z--n--m/a,--c=k",
			want:  map[string]string{"--h": "", "--a": "a/b,c/d--z--n--m/a", "--c": "k"},
		},
		{
			name:  "values with dash boolean flag in the middle",
			input: "--a=a/b,c/d--z--n--m/a,--h,--c=k",
			want:  map[string]string{"--a": "a/b,c/d--z--n--m/a", "--h": "", "--c": "k"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strsl := goflags.StringSlice{}
			for _, v := range strings.Split(tt.input, ",") {
				//nolint
				strsl.Set(v)
			}
			opt := Options{HeadlessOptionalArguments: strsl}
			got := opt.ParseHeadlessOptionalArguments()
			require.Equal(t, tt.want, got)
		})
	}
}
