// Package cmd implements the `generator` CLI, organized as subcommands:
//
//	generator encode  [flags] [content]
//	generator decode  [flags] <image>...
//	generator version
//	generator help
package cmd

import (
	"errors"
	"flag"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"runtime/debug"
)

// errReported marks an error whose message was already written to stderr (e.g.
// by the flag package). Exec exits non-zero without printing it again.
var errReported = errors.New("error already reported")

// Exec is the CLI entrypoint.
func Exec() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr); err != nil {
		if !errors.Is(err, errReported) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}

// run dispatches to a subcommand. The first argument is the command name.
func run(args []string, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		topUsage(stderr)
		return errReported
	}

	cmd, rest := args[0], args[1:]
	switch cmd {
	case "encode":
		return runEncode(rest, stdout, stderr)
	case "decode":
		return runDecode(rest, stdout, stderr)
	case "version", "-version", "--version":
		fmt.Fprintln(stdout, version())
		return nil
	case "help", "-h", "-help", "--help":
		topUsage(stdout)
		return nil
	default:
		fmt.Fprintf(stderr, "unknown command %q\n\n", cmd)
		topUsage(stderr)
		return errReported
	}
}

// topUsage prints the list of subcommands.
func topUsage(w io.Writer) {
	fmt.Fprint(w, `generator — encode/decode QR codes.

Usage:
  generator <command> [flags] [args]

Commands:
  encode     Encode text or a structured payload into QR image(s)
  decode     Decode QR code images into text
  version    Print version and exit
  help       Show this help

Run "generator <command> -h" for command-specific flags.

Examples:
  generator encode -png hello.png hello
  generator encode -module rounded -finder rounded -gradient '#1a237e,#00695c,45' -png styled.png -verify hello
  generator encode -payload wifi -png wifi.png "ssid=home,password=s3cret,auth=WPA"
  generator decode hello.png
`)
}

// parseFlags parses args. It reports help when -h was given, and returns
// errReported for flag errors, which the flag package has already printed.
func parseFlags(fs *flag.FlagSet, args []string) (help bool, err error) {
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return true, nil
		}
		return false, errReported
	}
	return false, nil
}

// setFlagsAmong returns which of the named flags were explicitly set, as "-name".
func setFlagsAmong(fs *flag.FlagSet, names ...string) []string {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		want[n] = true
	}
	var hit []string
	fs.Visit(func(f *flag.Flag) {
		if want[f.Name] {
			hit = append(hit, "-"+f.Name)
		}
	})
	return hit
}

// version reports the module version recorded in the build (a tag for
// `go install ...@vX`, or "(devel)" for local builds).
func version() string {
	if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" {
		return bi.Main.Version
	}
	return "(devel)"
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}
