// crawl-bench measures crawl coverage on real apps from the server side.
//
// Each target sits behind a recording reverse proxy, katana crawls the proxy,
// and the requests the app received are summarized into metrics. Results can be
// compared to a baseline from another build or flag set.
//
//	docker compose up -d --wait
//	go run . -katana ../../../katana -out new.json -baseline old.json
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"gopkg.in/yaml.v3"
)

type target struct {
	Name string `yaml:"name"`
	URL  string `yaml:"url"`
	// Prepare resets the target before every crawl; each line it prints is passed to katana as a -H header.
	Prepare string   `yaml:"prepare"`
	Args    []string `yaml:"args"`
}

type config struct {
	Targets []target `yaml:"targets"`
	// Runs maps a run name to katana flags, e.g. a standard and a headless crawl.
	Runs map[string][]string `yaml:"runs"`
}

// results maps target, then run name, to median metrics.
type results map[string]map[string]Metrics

func main() {
	configPath := flag.String("config", "bench.yaml", "benchmark config")
	katana := flag.String("katana", "katana", "katana binary to measure")
	repeat := flag.Int("repeat", 3, "crawls per target and run; the median is reported")
	only := flag.String("run", "", "comma separated run names to execute (default all)")
	out := flag.String("out", "", "write results as json")
	baseline := flag.String("baseline", "", "results json to compare against")
	flag.Parse()

	cfg, err := loadConfig(*configPath)
	if err != nil {
		fatal(err)
	}
	runs := selectRuns(cfg.Runs, *only)

	res := results{}
	for _, t := range cfg.Targets {
		res[t.Name] = map[string]Metrics{}
		for _, name := range sortedKeys(runs) {
			var samples []Metrics
			for i := 0; i < *repeat; i++ {
				m, err := crawl(*katana, t, runs[name])
				if err != nil {
					fatal(fmt.Errorf("%s/%s: %w", t.Name, name, err))
				}
				fmt.Fprintf(os.Stderr, "%s/%s #%d: %+v\n", t.Name, name, i+1, m)
				samples = append(samples, m)
			}
			res[t.Name][name] = median(samples)
		}
	}

	if *out != "" {
		data, _ := json.MarshalIndent(res, "", "  ")
		if err := os.WriteFile(*out, append(data, '\n'), 0o644); err != nil {
			fatal(err)
		}
	}
	var base results
	if *baseline != "" {
		data, err := os.ReadFile(*baseline)
		if err != nil {
			fatal(err)
		}
		if err := json.Unmarshal(data, &base); err != nil {
			fatal(err)
		}
	}
	report(res, base)
}

func crawl(katana string, t target, runArgs []string) (Metrics, error) {
	headers, err := prepare(t.Prepare)
	if err != nil {
		return Metrics{}, fmt.Errorf("prepare: %w", err)
	}
	upstream, err := url.Parse(t.URL)
	if err != nil {
		return Metrics{}, err
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return Metrics{}, err
	}
	rec := newRecorder(upstream)
	srv := &http.Server{Handler: rec, ReadHeaderTimeout: 10 * time.Second}
	go func() { _ = srv.Serve(ln) }()
	defer func() { _ = srv.Shutdown(context.Background()) }()

	args := []string{"-u", "http://" + ln.Addr().String() + "/", "-silent", "-disable-update-check"}
	for _, h := range headers {
		args = append(args, "-H", h)
	}
	args = append(append(args, runArgs...), t.Args...)

	cmd := exec.Command(katana, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	start := time.Now()
	if err := cmd.Run(); err != nil {
		return Metrics{}, fmt.Errorf("katana %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return computeMetrics(rec.snapshot(), time.Since(start).Seconds()), nil
}

func prepare(command string) ([]string, error) {
	if command == "" {
		return nil, nil
	}
	cmd := exec.Command("sh", "-c", command)
	cmd.Stderr = os.Stderr
	output, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	var headers []string
	scanner := bufio.NewScanner(bytes.NewReader(output))
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			headers = append(headers, line)
		}
	}
	return headers, nil
}

func report(res, base results) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "target\trun\t"+strings.Join(metricNames, "\t"))
	for _, tname := range sortedKeys(res) {
		for _, rname := range sortedKeys(res[tname]) {
			cur := res[tname][rname].fields()
			cells := make([]string, len(cur))
			old, ok := base[tname][rname]
			for i, v := range cur {
				cells[i] = trim(v)
				if ok {
					cells[i] += " (" + delta(old.fields()[i], v) + ")"
				}
			}
			_, _ = fmt.Fprintf(w, "%s\t%s\t%s\n", tname, rname, strings.Join(cells, "\t"))
		}
	}
	_ = w.Flush()
}

func trim(v float64) string {
	if v == float64(int(v)) {
		return fmt.Sprint(int(v))
	}
	return fmt.Sprintf("%.1f", v)
}

func loadConfig(path string) (*config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if len(cfg.Targets) == 0 || len(cfg.Runs) == 0 {
		return nil, fmt.Errorf("%s needs at least one target and one run", path)
	}
	return &cfg, nil
}

func selectRuns(all map[string][]string, only string) map[string][]string {
	if only == "" {
		return all
	}
	selected := map[string][]string{}
	for _, name := range strings.Split(only, ",") {
		args, ok := all[name]
		if !ok {
			fatal(fmt.Errorf("unknown run %q", name))
		}
		selected[name] = args
	}
	return selected
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
