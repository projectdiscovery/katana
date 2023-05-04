package testutils

import (
	"strings"

	errorutils "github.com/projectdiscovery/utils/errors"
)

type TestCase struct {
	Name        string
	Target      string
	Args        string
	Expected    []string
	CompareFunc func(target string, got []string) error
}

var TestCases = []TestCase{
	{
		Name: "Headless Browser Without Incognito",
		Target:   "https://www.hackerone.com/",
		Expected: nil,
		Args:     "-headless -no-incognito -depth 2 -silent",
		CompareFunc: func(target string, got []string) error {
			if strings.Contains(got[0], target) && len(got) > 10 {
				return nil
			}
			return errorutils.New("expected > 10 results, but got %v ", len(got))
		},
	},
}
