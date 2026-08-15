package types

import (
	"strings"
	"sync"
	"testing"

	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/gologger"
	"github.com/stretchr/testify/require"
)

// TestConfigureOutputConcurrent guards against a data race in the process-wide
// gologger instance: SetMaxLevel writes its level field without synchronization,
// so calling ConfigureOutput from multiple crawls at once (as a library consumer
// might) used to race against the level reads gologger performs on every log call.
// Run with -race to verify.
func TestConfigureOutputConcurrent(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			options := &Options{}
			switch n % 4 {
			case 0:
				options.Silent = true
			case 1:
				options.Verbose = true
			case 2:
				options.Debug = true
			}
			options.ConfigureOutput()
			gologger.Info().Msg("concurrent configure output test")
		}(i)
	}
	wg.Wait()
}

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
