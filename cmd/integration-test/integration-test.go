package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/logrusorgru/aurora"
)

type TestCase interface {
	// Execute executes a test case and returns any errors if occurred
	Execute() error
}

var (
	debug      = os.Getenv("DEBUG") == "true"
	customTest = os.Getenv("TEST")

	errored = false
	success = aurora.Green("[✓]").String()
	failed  = aurora.Red("[✘]").String()

	tests = map[string]map[string]TestCase{
		"code":    libraryTestcases,
		"filters": filtersTestcases,
	}
)

func main() {
	for name, tests := range tests {
		fmt.Printf("Running test cases for \"%s\"\n", aurora.Blue(name))
		if customTest != "" && !strings.Contains(name, customTest) {
			continue // only run tests user asked
		}
		for name, test := range tests {
			err := test.Execute()
			if err != nil {
				fmt.Fprintf(os.Stderr, "%s Test \"%s\" failed: %s\n", failed, name, err)
				errored = true
			} else {
				fmt.Printf("%s Test \"%s\" passed!\n", success, name)
			}
		}
	}
	if errored {
		os.Exit(1)
	}
}
