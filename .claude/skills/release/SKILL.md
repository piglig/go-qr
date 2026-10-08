---
name: release
description: Release the go-qr library, and the tools and mcp modules that follow it, through the release pull request, the release workflow and Dependabot. Use only when a maintainer asks for a release.
disable-model-invocation: true
---

# Release

Releases are made by `.github/workflows/release.yml`; tags are protected
and a hook blocks creating them by hand. This skill prepares and checks the
pull requests that trigger the workflow. The steps are in
[CONTRIBUTING.md](../../../CONTRIBUTING.md#releases).

## 1. Check that main is ready

1. `git checkout main && git pull`, with a clean working tree.
2. Read `## [Unreleased]` in `CHANGELOG.md`. Every user-visible change
   since the last tag (`git log --oneline $(git tag -l 'v*' | sort -V | tail -1)..HEAD`)
   must have an entry. Report missing or unclear entries to the maintainer
   instead of releasing.
3. Check the open pull requests (`gh pr list`). Ask whether any should go
   into this release.
4. Make sure the last CI run on main passed: `gh run list --branch main -L 3`.

## 2. Choose the version

Run `scripts/release.sh` without arguments. It prints the suggested version:
minor for Added, Changed, Deprecated or Removed entries, patch otherwise.
Tell the maintainer the suggestion and the reason, and wait for their
choice. Never release a new major version; that needs a new module path.

## 3. Open the release pull request

Run `scripts/release.sh X.Y.Z`. It checks the version and the docs'
*Since* markers, moves the entries under the new heading, and opens
`chore: release vX.Y.Z`. If it refuses, report why; do not work around it.

Wait for CI (`gh pr checks <number> --watch`). Merge only when the
maintainer says so: `gh pr merge <number> --squash --delete-branch`.

## 4. Confirm the release

1. Watch the workflow: `gh run list --workflow=release.yml -L 1`, then
   `gh run watch <id>`.
2. Confirm the release exists (`gh release view vX.Y.Z`) and that the
   module proxy has it:
   `GOPROXY=https://proxy.golang.org GOWORK=off go list -m github.com/piglig/go-qr/v2@vX.Y.Z`.

## 5. tools and mcp

Dependabot checks daily and opens `chore(deps): bump github.com/piglig/go-qr/v2 ...`
for `/tools` and `/mcp`. When it appears, check that CI passes, including
`test (tools and mcp on the pinned library)`, and merge it when the
maintainer agrees. The workflow then releases the next patch versions of
`tools` and `mcp`; confirm them as in step 4.

To release `tools` or `mcp` without a library release, for example for a
new CLI flag, the maintainer runs the release workflow by hand from the
Actions tab (`gh workflow run release.yml -f module=tools -f bump=minor`),
after confirming the version with them.

## If something fails

Do not create tags or releases by hand, and do not edit the tag rules.
Report the failing step and its log to the maintainer.
