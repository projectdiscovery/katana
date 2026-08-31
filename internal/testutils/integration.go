package testutils

import (
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// shellQuote quotes s for bash so Windows paths with backslashes and values
// containing @/: don't get mangled by bash -c.
func shellQuote(s string) string {
	s = filepath.ToSlash(s)
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func RunKatanaBinaryAndGetResults(target string, katanaBinary string, debug bool, args []string) ([]string, error) {
	quotedArgs := make([]string, 0, len(args))
	for _, a := range args {
		quotedArgs = append(quotedArgs, shellQuote(a))
	}

	cmdLine := fmt.Sprintf(`echo %s | %s %s`,
		shellQuote(target),
		shellQuote(katanaBinary),
		strings.Join(quotedArgs, " "),
	)
	if debug {
		fmt.Printf("cmd: %s\n", cmdLine)
	}

	cmd := exec.Command("bash", "-c", cmdLine)
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
