---
name: perf-check
description: Measure the speed, allocations and decode accuracy of the current go-qr changes against main on this machine, the way the project requires for hot-path and decoder changes. Use before opening a pull request that touches encoding, rendering or decoding, or when asked whether a change is slower.
---

# Performance and accuracy check

CI compares every pull request with its base, but its runners are noisy and
it holds no real photos. This skill produces the before-and-after numbers
that AGENTS.md asks for, on this machine.

## 1. Build the base in a worktree

Never check out base files into the working tree.

```shell
base=$(git merge-base HEAD origin/main)
git worktree add --detach ../go-qr-base "$base"
```

Both trees need the workspace for the tools module:
`go work init . ./tools` in each, if `go.work` does not exist.

## 2. Benchmarks

Pick the benchmarks the change can affect, for example `Decode` for the
decoder, `Encode|Segments` for encoding, `PNG|SVG|Image|Styled` for
rendering. Alternate the two versions so that drifts in the machine's speed
affect both, and run nothing else heavy meanwhile:

```shell
for i in $(seq 10); do
  (cd ../go-qr-base && go test -run='^$' -bench='<pattern>' -benchmem -count=1 .) >> /tmp/old.txt
  go test -run='^$' -bench='<pattern>' -benchmem -count=1 . >> /tmp/new.txt
done
(cd tools && go run ./regress bench /tmp/old.txt /tmp/new.txt)
```

`regress` prints a Markdown table and exits with status 1 on the same
regressions as CI: more allocations, or more than 25% slower.

## 3. Decode accuracy (decoder changes)

```shell
(cd ../go-qr-base/tools && go test -run=TestRobustness ./bench/ -sweep -sweep-native -sweep-out /tmp/old-sweep.json)
(cd tools && go test -run=TestRobustness ./bench/ -sweep -sweep-native -sweep-out /tmp/new-sweep.json)
(cd tools && go run ./regress sweep /tmp/old-sweep.json /tmp/new-sweep.json)
```

If the maintainer has the BoofCV dataset locally, also run `TestBoofCV` on
both versions (see `tools/bench/README.md`) and compare the decoded images.
Never copy the dataset into the repository.

## 4. Report

- Put the tables in the pull request description, stating the machine and
  how many runs each side had.
- Name every regression, even a small one, with its numbers. If a feature
  makes the default path clearly slower, say so and evaluate whether to
  drop it or make it opt-in. Leave the `accept-regression` label to the
  maintainer.
- Report only what you measured.

## 5. Clean up

```shell
git worktree remove ../go-qr-base
```
