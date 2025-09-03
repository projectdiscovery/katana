package testutils

import (
	"strings"

	"github.com/projectdiscovery/utils/errkit"
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
		Name:     "Headless Browser Without Incognito",
		Target:   "https://www.hackerone.com/",
		Expected: nil,
		Args:     "-headless -no-incognito -depth 2 -silent -no-sandbox",
		CompareFunc: func(target string, got []string) error {
			for _, res := range got {
				if strings.Contains(res, target) {
					return nil
				}
			}
			return errkit.Newf("expected %v target in output, but got %v ", target, strings.Join(got, "\n"))
		},
	},
}
