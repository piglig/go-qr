#!/usr/bin/env bash
# release.sh opens the pull request that releases the library: it moves the
# [Unreleased] entries of CHANGELOG.md under a new version heading. When the
# pull request is merged, .github/workflows/release.yml tags the merge
# commit and publishes the GitHub release.
#
# Usage: scripts/release.sh            print the suggested version
#        scripts/release.sh X.Y.Z      open the release pull request
#
# Needs git and the GitHub CLI (gh), run from a clean, up-to-date main.
set -euo pipefail

cd "$(git rev-parse --show-toplevel)"

die() { echo "release: $*" >&2; exit 1; }

command -v gh >/dev/null || die "needs the GitHub CLI (gh)"
[ "$(git branch --show-current)" = main ] || die "run from main"
[ -z "$(git status --porcelain)" ] || die "the working tree is not clean"
git fetch -q --tags origin main
[ "$(git rev-parse HEAD)" = "$(git rev-parse origin/main)" ] || die "main is not up to date with origin"

unreleased=$(awk '/^## \[Unreleased\]/ {on=1; next} /^## \[/ {on=0} on' CHANGELOG.md)
[ -n "$(echo "$unreleased" | tr -d '[:space:]')" ] || die "nothing under [Unreleased] in CHANGELOG.md"

latest=$(git tag -l 'v*' | sort -V | tail -1)
IFS=. read -r major minor patch <<< "${latest#v}"
# Keep a Changelog sections: additions and behavior changes are a minor
# release, fixes and performance work a patch. Breaking changes need a new
# major version, which is a new module path and not done by this script.
if echo "$unreleased" | grep -qE '^### (Added|Changed|Deprecated|Removed)'; then
	suggested="$major.$((minor + 1)).0"
else
	suggested="$major.$minor.$((patch + 1))"
fi

if [ $# -eq 0 ]; then
	echo "Latest release: $latest"
	echo "Sections under [Unreleased]: $(echo "$unreleased" | grep -E '^### ' | cut -c5- | paste -sd, -)"
	echo "Suggested version: $suggested"
	echo "Run: scripts/release.sh $suggested"
	exit 0
fi

v=${1#v}
echo "$v" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$' || die "version must be X.Y.Z"
[ "${v%%.*}" = "$major" ] || die "a new major version needs a new module path"
case "$v" in
"$major.$minor.$((patch + 1))" | "$major.$((minor + 1)).0") ;;
*) die "v$v does not follow $latest; use $major.$minor.$((patch + 1)) or $major.$((minor + 1)).0" ;;
esac
[ "$v" = "$suggested" ] || echo "release: note: $suggested was suggested from the changelog sections"

# Features in the guides are marked *Since vX.Y*; none may name a release
# after this one.
for since in $(grep -rhoE '\*Since v[0-9]+\.[0-9]+(\.[0-9]+)?' docs README.md | sort -u | sed 's/.*Since v//'); do
	[ "$(printf '%s\n%s\n' "$since" "$v" | sort -V | tail -1)" = "$v" ] ||
		die "docs mark a feature *Since v$since*, which is after v$v"
done

branch="release/v$v"
git switch -q -c "$branch"
heading="## [$v] - $(date +%Y-%m-%d)"
awk -v h="$heading" '{print} /^## \[Unreleased\]$/ {print ""; print h}' CHANGELOG.md > CHANGELOG.md.tmp
mv CHANGELOG.md.tmp CHANGELOG.md
git commit -q -am "chore: release v$v"
git push -q -u origin "$branch"

body=$(cat <<EOF
Moves the [Unreleased] CHANGELOG entries under v$v. Merging this pull
request tags the merge commit v$v and publishes the release
(\`.github/workflows/release.yml\`). Dependabot then opens the pull request
that builds tools and mcp on v$v.
EOF
)
gh pr create --base main --title "chore: release v$v" --body "$body"
