package testutils

import (
	"strings"

	"github.com/projectdiscovery/katana/internal/testutils/authlab"
	"github.com/projectdiscovery/utils/errkit"
)

// AuthTestCases returns functional cases that exercise recorded-flow login
// against a running authlab instance. flowPath must point at a Chrome Recorder
// (or explicit steps) JSON file whose navigate URL targets that lab.
func AuthTestCases(labURL, flowPath string) []TestCase {
	creds := authlab.Username + ":" + authlab.Password
	return []TestCase{
		{
			Name:   "Recorded Flow Authenticated Crawl",
			Target: labURL + "/app/dashboard",
			Args:   "-headless -no-incognito -depth 2 -silent -no-sandbox -rf " + flowPath + " -al " + creds,
			CompareFunc: func(_ string, got []string) error {
				joined := strings.Join(got, "\n")
				if !strings.Contains(joined, "/app/secret") && !strings.Contains(joined, "/app/settings") {
					return errkit.Newf("recorded flow crawl should discover gated /app pages, got: %s", joined)
				}
				return nil
			},
		},
		{
			Name:   "Recorded Flow Explicit Steps Crawl",
			Target: labURL + "/app/dashboard",
			Args:   "-headless -no-incognito -depth 2 -silent -no-sandbox -rf " + flowPath + " -al " + creds,
			CompareFunc: func(_ string, got []string) error {
				if len(got) == 0 {
					return errkit.New("expected authenticated crawl output")
				}
				return nil
			},
		},
	}
}
