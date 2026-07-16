package runner

import (
	"fmt"
	"net"
	"os/exec"
	"runtime"
	"strings"

	"github.com/projectdiscovery/goflags"
	"github.com/projectdiscovery/katana/pkg/types"
	fileutil "github.com/projectdiscovery/utils/file"
	permissionutil "github.com/projectdiscovery/utils/permission"
)

func DoHealthCheck(options *types.Options, flagSet *goflags.FlagSet) string {
	// RW permissions on config file
	cfgFilePath, _ := flagSet.GetConfigFilePath()
	var test strings.Builder
	_, _ = fmt.Fprintf(&test, "Version: %s\n", version)
	_, _ = fmt.Fprintf(&test, "Operative System: %s\n", runtime.GOOS)
	_, _ = fmt.Fprintf(&test, "Architecture: %s\n", runtime.GOARCH)
	_, _ = fmt.Fprintf(&test, "Go Version: %s\n", runtime.Version())
	_, _ = fmt.Fprintf(&test, "Compiler: %s\n", runtime.Compiler)

	var testResult string
	if permissionutil.IsRoot {
		testResult = "Ok"
	} else {
		testResult = "Ko"
	}
	_, _ = fmt.Fprintf(&test, "root: %s\n", testResult)

	ok, err := fileutil.IsReadable(cfgFilePath)
	if ok {
		testResult = "Ok"
	} else {
		testResult = "Ko"
	}
	if err != nil {
		testResult += fmt.Sprintf(" (%s)", err)
	}
	_, _ = fmt.Fprintf(&test, "Config file \"%s\" Read => %s\n", cfgFilePath, testResult)
	ok, err = fileutil.IsWriteable(cfgFilePath)
	if ok {
		testResult = "Ok"
	} else {
		testResult = "Ko"
	}
	if err != nil {
		testResult += fmt.Sprintf(" (%s)", err)
	}
	_, _ = fmt.Fprintf(&test, "Config file \"%s\" Write => %s\n", cfgFilePath, testResult)
	c4, err := net.Dial("tcp4", "scanme.sh:80")
	if err == nil && c4 != nil {
		_ = c4.Close()
	}
	testResult = "Ok"
	if err != nil {
		testResult = fmt.Sprintf("Ko (%s)", err)
	}
	_, _ = fmt.Fprintf(&test, "TCP IPv4 connectivity to scanme.sh:80 => %s\n", testResult)
	c6, err := net.Dial("tcp6", "scanme.sh:80")
	if err == nil && c6 != nil {
		_ = c6.Close()
	}
	testResult = "Ok"
	if err != nil {
		testResult = fmt.Sprintf("Ko (%s)", err)
	}
	_, _ = fmt.Fprintf(&test, "TCP IPv6 connectivity to scanme.sh:80 => %s\n", testResult)
	u4, err := net.Dial("udp4", "scanme.sh:53")
	if err == nil && u4 != nil {
		_ = u4.Close()
	}
	testResult = "Ok"
	if err != nil {
		testResult = fmt.Sprintf("Ko (%s)", err)
	}
	_, _ = fmt.Fprintf(&test, "UDP IPv4 connectivity to scanme.sh:80 => %s\n", testResult)
	u6, err := net.Dial("udp6", "scanme.sh:80")
	if err == nil && u6 != nil {
		_ = u6.Close()
	}
	testResult = "Ok"
	if err != nil {
		testResult = fmt.Sprintf("Ko (%s)", err)
	}
	_, _ = fmt.Fprintf(&test, "UDP IPv6 connectivity to scanme.sh:80 => %s\n", testResult)

	// attempt to identify if chome is installed locally
	if chromePath, err := exec.LookPath("chrome"); err == nil {
		_, _ = fmt.Fprintf(&test, "Potential chrome binary path (linux/osx) => %s\n", chromePath)
	}
	if chromePath, err := exec.LookPath("chrome.exe"); err == nil {
		_, _ = fmt.Fprintf(&test, "Potential chrome.exe binary path (windows) => %s\n", chromePath)
	}

	return test.String()
}
