package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

const benchOld = `goos: linux
BenchmarkDecode/v1-8   	    1000	    100000 ns/op	    2048 B/op	      20 allocs/op
BenchmarkDecode/v1-8   	    1000	    104000 ns/op	    2048 B/op	      20 allocs/op
BenchmarkDecode/v1-8   	    1000	     96000 ns/op	    2048 B/op	      20 allocs/op
BenchmarkPNG-8         	    2000	     50000 ns/op	    1024 B/op	       5 allocs/op
PASS
`

func TestCompareBench(t *testing.T) {
	cases := []struct {
		name, new string
		fail      []string
	}{
		{"unchanged", benchOld, nil},
		{"faster", strings.ReplaceAll(benchOld, "50000 ns/op", "30000 ns/op"), nil},
		{"slower", strings.ReplaceAll(benchOld, "50000 ns/op", "70000 ns/op"), []string{"PNG is 40% slower."}},
		{"within noise", strings.ReplaceAll(benchOld, "50000 ns/op", "60000 ns/op"), nil},
		{"more allocs", strings.ReplaceAll(benchOld, "20 allocs/op", "23 allocs/op"), []string{"Decode/v1 allocates 23 times per op instead of 20."}},
		{"allocs rounding", strings.ReplaceAll(benchOld, "20 allocs/op", "21 allocs/op"), nil},
		{"new benchmark", benchOld + "BenchmarkSVG-8 100 900 ns/op 10 B/op 1 allocs/op\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := compareBench(io.Discard, writeTemp(t, "old.txt", benchOld), writeTemp(t, "new.txt", c.new), 1.25)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, "|") != strings.Join(c.fail, "|") {
				t.Errorf("failures = %q, want %q", got, c.fail)
			}
		})
	}
}

func TestCompareBenchEmpty(t *testing.T) {
	if _, err := compareBench(io.Discard, writeTemp(t, "old.txt", benchOld), writeTemp(t, "new.txt", "PASS\n"), 1.25); err == nil {
		t.Error("no error for output without benchmarks")
	}
}

const sweepOld = `{"tilt=30": 32, "tilt=50": 20, "tilt wrong": 0, "blur=2": 30}`

func TestCompareSweep(t *testing.T) {
	cases := []struct {
		name, new string
		fail      []string
	}{
		{"unchanged", sweepOld, nil},
		{"better", `{"tilt=30": 32, "tilt=50": 25, "tilt wrong": 0, "blur=2": 30}`, nil},
		{"trade within tolerance", `{"tilt=30": 31, "tilt=50": 22, "tilt wrong": 0, "blur=2": 30}`, nil},
		{"point drops", `{"tilt=30": 32, "tilt=50": 17, "tilt wrong": 0, "blur=2": 34}`, []string{"tilt=50 decodes 17 images instead of 20."}},
		{"total drops", `{"tilt=30": 31, "tilt=50": 20, "tilt wrong": 0, "blur=2": 30}`, []string{"the sweeps decode 81 images in total instead of 82."}},
		{"wrong decode", `{"tilt=30": 32, "tilt=50": 20, "tilt wrong": 1, "blur=2": 30}`, []string{"tilt: 1 wrong decodes instead of 0."}},
		{"new point", `{"tilt=30": 32, "tilt=50": 20, "tilt wrong": 0, "blur=2": 30, "blur=3": 4}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := compareSweep(io.Discard, writeTemp(t, "old.json", sweepOld), writeTemp(t, "new.json", c.new), 1)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, "|") != strings.Join(c.fail, "|") {
				t.Errorf("failures = %q, want %q", got, c.fail)
			}
		})
	}
}
