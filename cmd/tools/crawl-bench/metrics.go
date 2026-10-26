package main

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/projectdiscovery/katana/pkg/utils"
)

// Metrics summarizes what one crawl made the target server do.
type Metrics struct {
	// Routes is distinct method and path, with ids normalized, that the server did not 404.
	Routes int `json:"routes"`
	// Params is distinct route and parameter name pairs from query strings and bodies.
	Params int `json:"params"`
	// Forms is distinct route and parameter sets sent in a request body.
	Forms int `json:"forms"`
	// Requests is every request the server received, duplicates included.
	Requests int `json:"requests"`
	// Errors is responses with a 5xx status.
	Errors  int     `json:"errors"`
	Seconds float64 `json:"seconds"`
}

func computeMetrics(hits []hit, seconds float64) Metrics {
	routes := map[string]struct{}{}
	params := map[string]struct{}{}
	forms := map[string]struct{}{}
	m := Metrics{Requests: len(hits), Seconds: seconds}

	for _, h := range hits {
		if h.Status >= http.StatusInternalServerError {
			m.Errors++
		}
		if h.Status == http.StatusNotFound {
			continue
		}
		route := h.Method + " " + utils.FingerprintURL(h.Path, nil)
		routes[route] = struct{}{}
		for _, p := range h.Params {
			params[route+" "+p] = struct{}{}
		}
		if h.Body {
			names := append([]string(nil), h.Params...)
			sort.Strings(names)
			forms[route+" "+strings.Join(names, ",")] = struct{}{}
		}
	}
	m.Routes, m.Params, m.Forms = len(routes), len(params), len(forms)
	return m
}

// median takes the per field median, so one unlucky run does not decide a comparison.
func median(runs []Metrics) Metrics {
	pick := func(get func(Metrics) float64) float64 {
		values := make([]float64, len(runs))
		for i, r := range runs {
			values[i] = get(r)
		}
		sort.Float64s(values)
		return values[len(values)/2]
	}
	return Metrics{
		Routes:   int(pick(func(m Metrics) float64 { return float64(m.Routes) })),
		Params:   int(pick(func(m Metrics) float64 { return float64(m.Params) })),
		Forms:    int(pick(func(m Metrics) float64 { return float64(m.Forms) })),
		Requests: int(pick(func(m Metrics) float64 { return float64(m.Requests) })),
		Errors:   int(pick(func(m Metrics) float64 { return float64(m.Errors) })),
		Seconds:  pick(func(m Metrics) float64 { return m.Seconds }),
	}
}

func (m Metrics) fields() []float64 {
	return []float64{float64(m.Routes), float64(m.Params), float64(m.Forms), float64(m.Requests), float64(m.Errors), m.Seconds}
}

var metricNames = []string{"routes", "params", "forms", "requests", "errors", "seconds"}

func delta(base, cur float64) string {
	if base == 0 {
		if cur == 0 {
			return "="
		}
		return "new"
	}
	pct := (cur - base) / base * 100
	if pct > -0.5 && pct < 0.5 {
		return "="
	}
	return fmt.Sprintf("%+.0f%%", pct)
}
