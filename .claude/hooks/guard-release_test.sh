#!/usr/bin/env bash
# Runs guard-release.sh on each case in guard-release.cases: the expected
# exit status (0 allowed, 2 blocked), a tab, and the command as it appears
# JSON-escaped in a tool call.
cd "$(dirname "$0")"
fail=0
while IFS=$'\t' read -r want c; do
	printf '{"tool_name":"Bash","tool_input":{"command":"%s"}}' "$c" | bash guard-release.sh 2>/dev/null
	got=$?
	if [ "$got" != "$want" ]; then
		echo "exit $got, want $want: $c"
		fail=1
	fi
done < guard-release.cases
exit $fail
