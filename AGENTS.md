# Instructions for AI coding assistants

These rules apply to every change made with an AI assistant in this
repository. [CONTRIBUTING.md](CONTRIBUTING.md) has the full contributor
guide; where the two differ, this file is the stricter one.

## Repository

| Module | Path | Notes |
| --- | --- | --- |
| `github.com/piglig/go-qr/v2` | `/` | The library. Go 1.23. Standard library only: never add a dependency. |
| `github.com/piglig/go-qr/tools` | `tools/` | `generator` CLI, `verify`, `bench` comparisons. Pins a released library version. |
| `github.com/piglig/go-qr/mcp` | `mcp/` | `go-qr-mcp` server. Go 1.25. Pins a released library version. |
| `github.com/piglig/go-qr/demo` | `demo/` | WebAssembly playground. |

To work across modules against the local library, create the ignored
workspace once: `go work init . ./tools ./demo ./mcp`.

## Before you call a change done

Run, from the repository root, and fix everything they report:

```shell
gofmt -l .                     # must print nothing
go vet ./...
go test ./...
(cd tools && go vet ./... && go test ./...)
(cd mcp && go vet ./... && go test ./...)
go run honnef.co/go/tools/cmd/staticcheck@latest ./...
```

- Every bug fix and behavior change comes with a test next to the code it
  covers. Exported API needs a doc comment and an example in
  `example_test.go`.
- When you touch the encoder, renderer, decoder or payload parser, run the
  matching fuzz target for at least a minute, for example
  `go test -run='^$' -fuzz='^FuzzDecodeNoPanic$' -fuzztime=1m .`.
- Never weaken, skip or delete a test to make a change pass.

## Performance and accuracy

CI compares every pull request with its base: benchmark time and
allocations, and the decode rates of the synthetic distortion sweeps (see
[CONTRIBUTING.md](CONTRIBUTING.md#performance-and-accuracy)). Passing it is
necessary, not sufficient:

- A change to a hot path (encoding, rendering, `Decode`) needs before and
  after numbers from the same machine in the pull request: several
  alternating runs of `go test -run='^$' -bench=<name> -benchmem`, compared
  with benchstat or `go run ./regress bench` in `tools`. Build the "before"
  side in a separate `git worktree`, not by checking out files.
- Run the sweeps for decoder changes locally, in `tools` with the
  workspace: `go test -run=TestRobustness -v ./bench/ -sweep`.
- If a feature makes the default path clearly slower, say so with numbers
  and evaluate whether to drop it or make it opt-in. Do not hide it, and
  never add the `accept-regression` label yourself; a maintainer decides.
- Report measured results only. If you did not run a benchmark, say so.

## Documentation and changelog

Update these in the same pull request as the code:

- the relevant guide under `docs/`, marking new features *Since vX.Y* with
  the next release;
- `CHANGELOG.md` under `## [Unreleased]`, in the existing style, for every
  `feat`, `fix` and `perf` change. CI fails without it.

## Pull requests

- One logical change per pull request. Titles follow Conventional Commits
  (`feat`, `fix`, `perf`, `refactor`, `docs`, `style`, `test`, `ci`,
  `chore`, optional scope, `!` for breaking changes); CI checks them.
- Pull requests are squash-merged; the title becomes the commit message.
- The description says what changed, why, how it was tested, and the
  performance impact.

## Releases

Never create tags or GitHub releases, and never push to `main`; tags are
protected and only the release workflow creates them. Start a release only
when a maintainer asks, and then only through the steps in
[CONTRIBUTING.md](CONTRIBUTING.md#releases):

- the library: `scripts/release.sh` to see the suggested version, then
  `scripts/release.sh X.Y.Z` to open the release pull request;
- `tools` and `mcp`: merging the Dependabot pull request that bumps their
  library pin releases them. Do not add CHANGELOG entries for these bumps;
  their releases carry their own notes.

## Claude Code

The project settings in `.claude/` add:

- `/release`, the release steps, for a maintainer to start;
- `/perf-check`, the before-and-after measurement for hot-path and decoder
  changes;
- a hook that blocks creating tags or releases and pushing to `main`.

Other assistants follow the same rules from this file.

## Data and licenses

- Do not vendor test images or datasets whose license is unclear, such as
  the BoofCV QR Code dataset; tests download or point to them instead.
- Do not copy code from other QR Code libraries. Follow the standard
  (ISO/IEC 18004) and published papers, and cite them.
