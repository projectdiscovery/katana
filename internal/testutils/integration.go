package testutils

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

func RunKatanaBinaryAndGetResults(target string, katanaBinary string, debug bool, args []string) ([]string, error) {
	if debug {
		fmt.Printf("cmd: echo %s | %s %s\n", target, katanaBinary, strings.Join(args, " "))
	}

	cmd := exec.Command(katanaBinary, args...)
	cmd.Stdin = strings.NewReader(target + "\n")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("katana failed: %w\nstderr:\n%s\nstdout:\n%s", err, stderr.String(), stdout.String())
	}

	parts := []string{}
	for _, i := range strings.Split(stdout.String(), "\n") {
		if i != "" {
			parts = append(parts, i)
		}
	}
	return parts, nil
}
