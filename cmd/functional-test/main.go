package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/internal/testutils"
)

var (
	debug           = os.Getenv("DEBUG") == "true"
	success         = aurora.Green("[✓]").String()
	failed          = aurora.Red("[✘]").String()
	errored         = false
	devKatanaBinary = flag.String("dev", "", "Dev Branch Katana Binary")
)

func main() {
	flag.Parse()
	if err := runFunctionalTests(); err != nil {
		log.Fatalf("Could not run functional tests: %s\n", err)
	}
	if errored {
		os.Exit(1)
	}
}

func runFunctionalTests() error {
	for _, testcase := range testutils.TestCases {
		if err := runIndividualTestCase(testcase); err != nil {
			errored = true
			fmt.Fprintf(os.Stderr, "%s Test \"%s\" failed: %s\n", failed, testcase.Name, err)
		} else {
			fmt.Printf("%s Test \"%s\" passed!\n", success, testcase.Name)
		}
	}
	return nil
}

func runIndividualTestCase(testcase testutils.TestCase) error {
	argsParts := strings.Fields(testcase.Args)
	devOutput, err := testutils.RunKatanaBinaryAndGetResults(testcase.Target, *devKatanaBinary, debug, argsParts)
	if err != nil {
		return errors.Wrap(err, "could not run Katana dev test")
	}
	if testcase.CompareFunc != nil {
		return testcase.CompareFunc(testcase.Target, devOutput)
	}
	if !testutils.CompareOutput(devOutput, testcase.Expected) {
		return errors.Errorf("expected output %s, got %s", testcase.Expected, devOutput)
	}
	return nil
}
