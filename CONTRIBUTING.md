# Contributing to go-qr

Thank you for helping. Bug reports, documentation fixes, new tests and code
are all welcome. This guide explains how the repository is organized and
what a change needs before it can be merged.

## Before you start

- **Bugs:** open an [issue](https://github.com/piglig/go-qr/issues/new/choose)
  with the smallest program or input that reproduces the problem, plus the
  output of `go version` and `go list -m github.com/piglig/go-qr/v2`.
- **Features and API changes:** open an issue first to agree on the design.
  The public API follows semantic versioning, so additions are easy and
  changes are expensive.
- **Security issues:** do not open a public issue; see [SECURITY.md](SECURITY.md).

## Repository layout

| Path | Module | Contents |
| --- | --- | --- |
| `/` | `github.com/piglig/go-qr/v2` | The library. No dependencies outside the standard library. |
| `payload/` | same | Structured payload builders and parser. |
| `internal/reedsolomon/` | same | GF(2⁸) Reed–Solomon encoding and correction. |
| `tools/` | `github.com/piglig/go-qr/tools` | The `generator` CLI, `verify` helpers and the `bench` comparisons with other libraries. |
| `demo/` | `github.com/piglig/go-qr/demo` | The WebAssembly playground published to GitHub Pages. |
| `docs/` | | Documentation; see [docs/README.md](docs/README.md). |

[How it works](docs/explanation/how-it-works.md) maps the library's source
files to the encode, render and decode pipelines.

`tools` pins a released version of the library so that
`go install .../generator@latest` works. To develop against your checkout,
create a workspace once at the repository root:

```shell
go work init . ./tools ./demo
```

`go.work` is ignored by git.

## Development

Requirements: Go 1.23 or later; the CI also tests the two latest Go
releases.

```shell
go test ./...                     # library
(cd tools && go test ./...)       # CLI, verify and benchmark harness
(cd demo && go test ./...)        # playground logic
go vet ./...
gofmt -l .                        # must print nothing
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
```

### Tests

- Add a test for every bug fix and behavior change, next to the code it
  covers (`encode_test.go` for `encode.go`, and so on).
- Prefer table-driven tests and the small assertion helpers in
  `assert_test.go`; the module does not use third-party test libraries.
- Encoding changes should round-trip through the decoder. Rendering changes
  should check pixels or SVG content, and run `Code.Verify` for anything
  that affects readability.
- Public API needs a [testable example](https://go.dev/blog/examples) in
  `example_test.go`; examples are shown on pkg.go.dev and checked by
  `go test`.

### Golden files

`testdata/golden/*.svg` pin the exact SVG output. If you change SVG output
on purpose, regenerate them and review the diff:

```shell
go test -run TestGoldenSVG -update .
git diff testdata/golden
```

### Fuzzing

Every encoder, renderer, decoder and parser entry point has a fuzz target
(`FuzzEncode`, `FuzzEncodeStructured`, `FuzzRender`, `FuzzDecodeRoundTrip`,
`FuzzDecodeNoPanic`, `payload.FuzzParse`). CI runs each for 15 seconds. When
you touch one of these areas, run the relevant target for longer:

```shell
go test -run='^$' -fuzz='^FuzzEncode$' -fuzztime=2m .
```

Commit any failing input that the fuzzer writes under `testdata/fuzz/` as a
regression test, together with the fix.

### Performance

Changes to hot paths need before-and-after numbers. Benchmark both versions
on the same machine, several runs each, for example with
[benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```shell
go test -run='^$' -bench=. -benchmem -count=10 . > new.txt
```

Comparisons with other libraries are in [tools/bench](tools/bench); see
[Performance](docs/performance.md). A change that makes the default path
noticeably slower needs a strong reason, or should be opt-in.

### Documentation

Update the documentation in the same pull request as the code:

- the doc comment of every exported identifier you add or change;
- the relevant guide in `docs/guides/`, marking new features with
  *Since vX.Y*;
- `CHANGELOG.md` under `[Unreleased]`, following
  [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).

The playground in `demo/` can be run locally:

```shell
cd demo
GOOS=js GOARCH=wasm go build -o web/qr.wasm .
cp "$(go env GOROOT)/lib/wasm/wasm_exec.js" web/
go run .          # serves http://localhost:8080
```

## Pull requests

- Keep each pull request to one logical change; large features can be split
  into several.
- Titles follow [Conventional Commits](https://www.conventionalcommits.org/),
  which CI checks: `feat:`, `fix:`, `refactor:`, `docs:`, `style:`,
  `test:` or `chore:`, with `!` for breaking changes, for example
  `fix: keep GS1 separators unambiguous`.
- Describe what changed and why, how you tested it, and any performance
  impact.
- CI must pass: tests with `-race` on several Go versions, vet, gofmt,
  staticcheck, govulncheck, fuzzing and coverage.

Pull requests are squash-merged, so the title becomes the commit message.

## Releases

Maintainers release from `main`:

1. Move the `[Unreleased]` CHANGELOG entries under a new version heading,
   in a `chore: release vX.Y.Z` pull request.
2. After it merges, tag the merge commit `vX.Y.Z`, push the tag and create a
   GitHub release from the CHANGELOG section.
3. To ship library changes in the CLI, bump the library version in
   `tools/go.mod` in a separate pull request and then tag `tools/vX.Y.Z`.
   Always release the library first: the tools module must build against a
   published version.

## Code of conduct

This project follows the [Code of Conduct](CODE_OF_CONDUCT.md). By
participating you agree to uphold it.
