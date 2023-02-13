package runner

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/utils"
	"github.com/projectdiscovery/gologger"
	errorutil "github.com/projectdiscovery/utils/errors"
	"github.com/remeh/sizedwaitgroup"
)

// ExecuteCrawling executes the crawling main loop
func (r *Runner) ExecuteCrawling() error {
	if r.options.NewProject != "" {
		r.setupNewProject()
		os.Exit(0)
	}

	inputs := r.parseInputs()
	if len(inputs) == 0 {
		return errorutil.New("no input provided for crawling")
	}

	defer r.crawler.Close()

	wg := sizedwaitgroup.New(r.options.Parallelism)
	for _, input := range inputs {
		wg.Add()

		go func(input string) {
			defer wg.Done()

			if err := r.crawler.Crawl(input); err != nil {
				gologger.Warning().Msgf("Could not crawl %s: %s", input, err)
			}
		}(input)
	}
	wg.Wait()
	return nil
}

// setupNewProject opens browser for manual authentication
func (r *Runner) setupNewProject() {
	// create manager instance which manages browser
	manager := launcher.NewManager()

	// setup manager port
	// get a random port without preference
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		gologger.Fatal().Label("project").Msgf("failed to setup listener for manager got %v", err)
	}
	managerPort := 9000
	if value, ok := (listener.Addr()).(*net.TCPAddr); ok {
		managerPort = value.Port
	}

	//start manager goroutine
	go func() {
		log.Fatal(http.Serve(listener, manager))
	}()

	//open browser
	go func() {
		chromeLauncher, err := launcher.NewManaged("ws://127.0.0.1:" + strconv.Itoa(managerPort))
		if err != nil {
			panic(err)
		}
		chromeLauncher.
			Leakless(true).
			Set("disable-gpu", "true").
			Set("ignore-certificate-errors", "true").
			Set("ignore-certificate-errors", "1").
			Set("disable-crash-reporter", "true").
			Set("disable-notifications", "true").
			Set("hide-scrollbars", "true").
			Set("window-size", fmt.Sprintf("%d,%d", 1080, 1920)).
			Set("mute-audio", "true").
			Delete("use-mock-keychain").
			UserDataDir(r.options.NewProject).
			KeepUserDataDir().
			Headless(false)

		if r.options.UseInstalledChrome {
			if chromePath, hasChrome := launcher.LookPath(); hasChrome {
				chromeLauncher.Bin(chromePath)
			} else {
				gologger.Fatal().Label("project").Msgf("chrome browser is not installed")
			}
		}
		if r.options.SystemChromePath != "" {
			chromeLauncher.Bin(r.options.SystemChromePath)
		}
		if r.options.HeadlessNoSandbox {
			chromeLauncher.Set("no-sandbox", "true")
		}
		if r.options.Proxy != "" && r.options.Headless {
			proxyURL, err := url.Parse(r.options.Proxy)
			if err != nil {
				gologger.Fatal().Label("project").Msgf("failed to parse proxy url got %v", err)
			}
			chromeLauncher.Set("proxy-server", proxyURL.String())
		}

		if _, err := chromeLauncher.Launch(); err != nil {
			gologger.Fatal().Label("project").Msgf("failed to launch chromium got %v", err)
		}

		utils.Pause()
	}()
	fmt.Println("Started katana in New Project Mode. Follow below steps to complete creating new project")
	fmt.Println("1. You should now see a chromium window, if not locate it")
	fmt.Println("2. Login to your desired target in browser")
	fmt.Println("3. [Press Enter Key] to complete setup")

	// read one char from stdin
	fmt.Scanln()
	gologger.Verbose().Msgf("new project setup completed")
}
