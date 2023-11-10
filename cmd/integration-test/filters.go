package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

var filtersTestcases = map[string]TestCase{
	"match condition":  &matchConditionIntegrationTest{},
	"filter condition": &filterConditionIntegrationTest{},
}

type matchConditionIntegrationTest struct{}

// Execute executes a test case and returns an error if occurred
// Execute the docs at ../README.md if the code stops working for integration.
func (h *matchConditionIntegrationTest) Execute() error {
	results, _ := RunKatanaAndGetResults(false,
		"-u", "scanme.sh",
		"-match-condition", "status_code == 200 && contains(body, 'ok')",
	)

	if len(results) != 1 {
		return fmt.Errorf("match condition failed")
	}
	return nil
}

type filterConditionIntegrationTest struct{}

// Execute executes a test case and returns an error if occurred
// Execute the docs at ../README.md if the code stops working for integration.
func (h *filterConditionIntegrationTest) Execute() error {
	results, _ := RunKatanaAndGetResults(false,
		"-u", "scanme.sh",
		"-filter-condition", "status_code == 200 && contains(body, 'ok')",
	)

	if len(results) != 0 {
		return fmt.Errorf("filter condition failed")
	}
	return nil
}

// ExtraArgs
var ExtraDebugArgs = []string{}

func RunKatanaAndGetResults(debug bool, extra ...string) ([]string, error) {
	cmd := exec.Command("./katana")
	extra = append(extra, ExtraDebugArgs...)
	cmd.Args = append(cmd.Args, extra...)
	cmd.Args = append(cmd.Args, "-duc") // disable auto updates
	if debug {
		cmd.Args = append(cmd.Args, "-debug")
		cmd.Stderr = os.Stderr
		fmt.Println(cmd.String())
	} else {
		cmd.Args = append(cmd.Args, "-silent")
	}
	data, err := cmd.Output()
	if debug {
		fmt.Println(string(data))
	}
	if len(data) < 1 && err != nil {
		return nil, fmt.Errorf("%v: %v", err.Error(), string(data))
	}
	var parts []string
	items := strings.Split(string(data), "\n")
	for _, i := range items {
		if i != "" {
			parts = append(parts, i)
		}
	}
	return parts, nil
}
