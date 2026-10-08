// Command regress compares two versions of the library for the regression
// checks in CI, and prints the comparison as a Markdown table.
//
//	regress bench old.txt new.txt   compare `go test -bench -benchmem` outputs
//	regress sweep old.json new.json compare `TestRobustness -sweep-out` counts
//
// It exits with status 1 when the new version regresses: a benchmark that
// allocates more, or is slower by more than -max-slowdown, or a sweep point
// that decodes fewer images or more wrongly.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func main() {
	maxSlowdown := flag.Float64("max-slowdown", 1.25, "bench: fail when a benchmark's median time grows by more than this factor")
	tolerance := flag.Int("tolerance", 1, "sweep: decodes a single sweep point may lose, as long as the total does not drop")
	flag.Usage = func() {
		fmt.Fprintln(os.Stderr, "usage: regress [flags] bench|sweep old new")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 3 {
		flag.Usage()
		os.Exit(2)
	}
	var (
		failures []string
		err      error
	)
	switch flag.Arg(0) {
	case "bench":
		failures, err = compareBench(os.Stdout, flag.Arg(1), flag.Arg(2), *maxSlowdown)
	case "sweep":
		failures, err = compareSweep(os.Stdout, flag.Arg(1), flag.Arg(2), *tolerance)
	default:
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "regress:", err)
		os.Exit(2)
	}
	if len(failures) > 0 {
		fmt.Println()
		for _, f := range failures {
			fmt.Println("- **Regression:**", f)
		}
		os.Exit(1)
	}
}

// sample is one benchmark result line.
type sample struct{ ns, bytes, allocs float64 }

var procSuffix = regexp.MustCompile(`-\d+$`)

// parseBench reads `go test -bench` output into the samples of each
// benchmark, in order of first appearance.
func parseBench(path string) (map[string][]sample, []string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer f.Close()
	runs := map[string][]sample{}
	var names []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) < 4 || !strings.HasPrefix(fields[0], "Benchmark") {
			continue
		}
		var s sample
		for i := 2; i+1 < len(fields); i += 2 {
			v, err := strconv.ParseFloat(fields[i], 64)
			if err != nil {
				break
			}
			switch fields[i+1] {
			case "ns/op":
				s.ns = v
			case "B/op":
				s.bytes = v
			case "allocs/op":
				s.allocs = v
			}
		}
		name := procSuffix.ReplaceAllString(strings.TrimPrefix(fields[0], "Benchmark"), "")
		if _, ok := runs[name]; !ok {
			names = append(names, name)
		}
		runs[name] = append(runs[name], s)
	}
	return runs, names, sc.Err()
}

func median(xs []sample, get func(sample) float64) float64 {
	v := make([]float64, len(xs))
	for i, x := range xs {
		v[i] = get(x)
	}
	sort.Float64s(v)
	n := len(v)
	if n%2 == 1 {
		return v[n/2]
	}
	return (v[n/2-1] + v[n/2]) / 2
}

func compareBench(w io.Writer, oldPath, newPath string, maxSlowdown float64) ([]string, error) {
	oldRuns, _, err := parseBench(oldPath)
	if err != nil {
		return nil, err
	}
	newRuns, names, err := parseBench(newPath)
	if err != nil {
		return nil, err
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no benchmarks in %s", newPath)
	}
	ns := func(s sample) float64 { return s.ns }
	allocs := func(s sample) float64 { return s.allocs }
	bytes := func(s sample) float64 { return s.bytes }

	fmt.Fprintln(w, "| Benchmark | time before | time after | Δ | allocs before | allocs after | B/op after |")
	fmt.Fprintln(w, "| --- | ---: | ---: | ---: | ---: | ---: | ---: |")
	var failures []string
	logSum, compared := 0.0, 0
	for _, name := range names {
		nw := newRuns[name]
		old, ok := oldRuns[name]
		if !ok {
			fmt.Fprintf(w, "| %s | new | %s | | | %.0f | %.0f |\n", name, formatNs(median(nw, ns)), median(nw, allocs), median(nw, bytes))
			continue
		}
		on, nn := median(old, ns), median(nw, ns)
		oa, na := median(old, allocs), median(nw, allocs)
		ratio := nn / on
		logSum += math.Log(ratio)
		compared++
		fmt.Fprintf(w, "| %s | %s | %s | %+.1f%% | %.0f | %.0f | %.0f |\n",
			name, formatNs(on), formatNs(nn), 100*(ratio-1), oa, na, median(nw, bytes))
		// Allocation counts vary little between runs, so an increase is a
		// regression; the slack absorbs pools and caches whose cost is
		// spread over a different number of iterations.
		if na > oa+math.Max(2, 0.05*oa) {
			failures = append(failures, fmt.Sprintf("%s allocates %.0f times per op instead of %.0f.", name, na, oa))
		}
		if ratio > maxSlowdown {
			failures = append(failures, fmt.Sprintf("%s is %.0f%% slower.", name, 100*(ratio-1)))
		}
	}
	if compared > 0 {
		fmt.Fprintf(w, "\nGeometric mean of the time change: %+.1f%% over %d benchmarks.\n",
			100*(math.Exp(logSum/float64(compared))-1), compared)
	}
	return failures, nil
}

func formatNs(ns float64) string {
	switch {
	case ns >= 1e6:
		return fmt.Sprintf("%.2f ms", ns/1e6)
	case ns >= 1e3:
		return fmt.Sprintf("%.1f µs", ns/1e3)
	}
	return fmt.Sprintf("%.0f ns", ns)
}

func readCounts(path string) (map[string]int, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m map[string]int
	return m, json.Unmarshal(b, &m)
}

func compareSweep(w io.Writer, oldPath, newPath string, tolerance int) ([]string, error) {
	old, err := readCounts(oldPath)
	if err != nil {
		return nil, err
	}
	nw, err := readCounts(newPath)
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(nw))
	for k := range nw {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Fprintln(w, "| Sweep point | decoded before | decoded after |")
	fmt.Fprintln(w, "| --- | ---: | ---: |")
	var failures []string
	oldTotal, newTotal := 0, 0
	for _, k := range keys {
		o, ok := old[k]
		n := nw[k]
		wrong := strings.HasSuffix(k, " wrong")
		switch {
		case !ok:
			fmt.Fprintf(w, "| %s | new | %d |\n", k, n)
			continue
		case o != n:
			fmt.Fprintf(w, "| **%s** | %d | **%d** |\n", k, o, n)
		default:
			fmt.Fprintf(w, "| %s | %d | %d |\n", k, o, n)
		}
		if wrong {
			if n > o {
				failures = append(failures, fmt.Sprintf("%s: %d wrong decodes instead of %d.", strings.TrimSuffix(k, " wrong"), n, o))
			}
			continue
		}
		oldTotal += o
		newTotal += n
		if n < o-tolerance {
			failures = append(failures, fmt.Sprintf("%s decodes %d images instead of %d.", k, n, o))
		}
	}
	fmt.Fprintf(w, "\nDecoded in total: %d before, %d after.\n", oldTotal, newTotal)
	if newTotal < oldTotal {
		failures = append(failures, fmt.Sprintf("the sweeps decode %d images in total instead of %d.", newTotal, oldTotal))
	}
	return failures, nil
}
