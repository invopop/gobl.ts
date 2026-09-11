#!/usr/bin/env bash
#
# Exercises next-version.sh against synthetic tag histories.
#
# Each case builds a throwaway git repository with a given set of tags and
# stubs `go list -m` so the test controls the resolved GOBL version. Release
# numbering is derived rather than written down anywhere, so it needs to be
# tested: an off-by-one here either skips a version or tries to republish one
# npm has already taken.
#
#   .github/scripts/next-version.test.sh .github/scripts/next-version.sh
#
set -uo pipefail
SRC="$(cd "$(dirname "$1")" && pwd)/$(basename "$1")"
pass=0; fail=0

run_case() {
  local desc="$1" core="$2" tags="$3" expect="$4"
  local dir; dir="$(mktemp -d)"
  ( cd "$dir"
    git init -q . && git -c user.email=t@t -c user.name=t commit -q --allow-empty -m x
    for t in $tags; do git tag "$t"; done
    # Stub `go list -m` so the case controls the resolved core version.
    mkdir -p bin
    printf '#!/usr/bin/env bash\necho "%s"\n' "$core" > bin/go
    chmod +x bin/go
    PATH="$dir/bin:$PATH" bash "$SRC" 2>/dev/null
  ) > "$dir.out" 2>/dev/null
  local rc=$? got; got="$(cat "$dir.out")"; rm -f "$dir.out"
  rm -rf "$dir"

  if [[ "$expect" == "FAIL" ]]; then
    if [[ $rc -ne 0 ]]; then echo "  ok   $desc (rejected)"; ((pass++));
    else echo "  FAIL $desc: expected failure, got '$got'"; ((fail++)); fi
  elif [[ "$got" == "$expect" ]]; then
    echo "  ok   $desc -> $got"; ((pass++))
  else
    echo "  FAIL $desc: expected '$expect', got '$got' (rc=$rc)"; ((fail++))
  fi
}

echo "next-version.sh"
run_case "first release on a new GOBL line"      "v0.505.0" ""                          "v0.505.0"
run_case "increments the patch"                  "v0.505.0" "v0.505.0"                  "v0.505.1"
run_case "picks the highest, not the newest tag" "v0.505.0" "v0.505.0 v0.505.9 v0.505.2" "v0.505.10"
run_case "GOBL minor bump resets the patch"      "v0.506.0" "v0.505.0 v0.505.7"         "v0.506.0"
run_case "GOBL patch absorbed into our patch"    "v0.505.3" "v0.505.4"                  "v0.505.5"
run_case "ignores tags off this line"            "v0.505.0" "v0.504.9 v1.0.0"           "FAIL"
run_case "ignores non-release tags"              "v0.505.0" "v0.505.0-rc.0 v0.505.0"    "v0.505.1"
run_case "refuses to move backwards"             "v0.504.0" "v0.505.0"                  "FAIL"
run_case "rejects a bad core version"            "devel"    ""                          "FAIL"

echo "$pass passed, $fail failed"
[[ $fail -eq 0 ]]
