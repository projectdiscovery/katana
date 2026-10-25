package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/logrusorgru/aurora"
	"github.com/pkg/errors"
	"github.com/projectdiscovery/katana/internal/testutils"
	"github.com/projectdiscovery/katana/internal/testutils/authlab"
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
	server, err := testutils.NewTestServer()
	if err != nil {
		return fmt.Errorf("could not start test server: %w", err)
	}
	defer func() {
		_ = server.Close()
	}()

	fmt.Printf("Test server started at %s\n", server.URL)

	for _, testcase := range testutils.TestCases {
		if testcase.Target == "" {
			testcase.Target = server.URL
		}
		if err := runIndividualTestCase(testcase); err != nil {
			errored = true
			fmt.Fprintf(os.Stderr, "%s Test \"%s\" failed: %s\n", failed, testcase.Name, err)
		} else {
			fmt.Printf("%s Test \"%s\" passed!\n", success, testcase.Name)
		}
	}

	if err := runAuthLabFunctionalTests(); err != nil {
		return err
	}
	return nil
}

func runAuthLabFunctionalTests() error {
	lab, err := authlab.Start()
	if err != nil {
		return fmt.Errorf("could not start auth lab: %w", err)
	}
	defer func() { _ = lab.Close() }()

	fmt.Printf("Auth lab started at %s\n", lab.URL)

	dir, err := os.MkdirTemp("", "katana-rf-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	simpleFlow := filepath.Join(dir, "simple.json")
	if err := os.WriteFile(simpleFlow, []byte(authlab.ChromeRecordingSimple(lab.URL)), 0o600); err != nil {
		return err
	}
	explicitFlow := filepath.Join(dir, "explicit.json")
	if err := os.WriteFile(explicitFlow, []byte(authlab.ExplicitStepsSimple(lab.URL)), 0o600); err != nil {
		return err
	}

	cases := testutils.AuthTestCases(lab.URL, simpleFlow)
	// Second case reuses the same CompareFunc shape but should run the explicit steps file.
	if len(cases) >= 2 {
		cases[1].Args = strings.Replace(cases[1].Args, simpleFlow, explicitFlow, 1)
	}

	for _, testcase := range cases {
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
