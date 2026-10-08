#!/usr/bin/env bash
# guard-release.sh is a Claude Code PreToolUse hook. It blocks shell
# commands that create tags or releases or push to main, which only the
# release workflow and merged pull requests may do (see AGENTS.md).
#
# Claude Code passes the tool call as JSON on stdin; exit status 2 blocks the
# call and shows stderr to the model.
input=$(cat)

block() {
	echo "Blocked by .claude/hooks/guard-release.sh: $1" >&2
	echo "Tags and releases are made by .github/workflows/release.yml, and main changes only through merged pull requests. Use the /release skill when a maintainer asks for a release." >&2
	exit 2
}

# Look only at the command, not at other fields of the tool call.
cmd=$(printf '%s' "$input" | sed -n 's/.*"command"[[:space:]]*:[[:space:]]*"\(\([^"\\]\|\\.\)*\)".*/\1/p')
[ -n "$cmd" ] || exit 0
# Drop quoted arguments that span lines, such as commit messages and pull
# request bodies: they are text, which may mention these commands.
cmd=$(printf '%s' "$cmd" | sed -E 's/\\"([^\\]|\\[^"])*\\n([^\\]|\\[^"])*\\"/""/g')

# Creating, moving or deleting a tag; listing tags is fine.
if printf '%s' "$cmd" | grep -qE '(^|[;&|( ])git( -C [^ ]+)? tag [^-]|(^|[;&|( ])git( -C [^ ]+)? tag -(a|s|f|d|m|u|-annotate|-sign|-force|-delete)\b'; then
	block "creating or changing a git tag"
fi
# Pushing tags.
if printf '%s' "$cmd" | grep -qE 'git( -C [^ ]+)? push\b.*(--tags|--follow-tags|refs/tags| v[0-9]| (tools|mcp)/v[0-9])'; then
	block "pushing a tag"
fi
# Pushing to main.
if printf '%s' "$cmd" | grep -qE 'git( -C [^ ]+)? push\b[^;&|]*[ :]main([ ;&|)]|$)'; then
	block "pushing to main"
fi
# Creating or changing GitHub releases.
if printf '%s' "$cmd" | grep -qE 'gh(\.exe)?(\\")? release (create|edit|delete|upload)'; then
	block "creating or changing a GitHub release"
fi
exit 0
