#!/usr/bin/env bash
#
# Derive the next @invopop/gobl release tag.
#
# The package pins its major and minor to the github.com/invopop/gobl release
# the types were generated from, and owns the patch component for its own
# releases — the same scheme gobl.dev uses:
#
#   GOBL 0.505.x  ->  v0.505.0, v0.505.1, v0.505.2, ...
#   GOBL 0.506.0  ->  v0.506.0, v0.506.1, ...
#
# A GOBL patch release is absorbed into the next patch here: if we are already
# at v0.505.4 when GOBL 0.505.1 lands, the release carrying it is v0.505.5. The
# core patch is therefore not recoverable from the tag alone; it is recorded in
# the annotated tag message and in the generated `GOBL_VERSION` constant, which
# reports it at runtime.
#
# Prints the next tag on stdout. Exits non-zero rather than emit a tag that
# would move the version backwards.
#
set -euo pipefail

RELEASE_RE='^v[0-9]+\.[0-9]+\.[0-9]+$'

# `go list -m` reports the version MVS actually selected, so the tag follows the
# core version really used even when gobl.dev (required for the registry pass)
# raises it above our own require line. It also honours a replace directive,
# which grepping go.mod would not.
core="$(go list -m -f '{{.Version}}' github.com/invopop/gobl)"
if [[ ! "$core" =~ ^v[0-9]+\.[0-9]+\.[0-9]+ ]]; then
	echo "next-version: unexpected core version '$core'" >&2
	exit 1
fi

line="${core#v}"; line="${line%.*}" # v0.505.1 -> 0.505
line_re="${line//./\\.}"            # 0.505    -> 0\.505

# Highest release tag already on this core line.
latest="$(git tag -l "v${line}.*" | grep -E "^v${line_re}\.[0-9]+$" | sort -V | tail -n1 || true)"
if [[ -z "$latest" ]]; then
	next="v${line}.0" # first release on a new core line
else
	next="v${line}.$((${latest##*.} + 1))"
fi

# npm refuses to republish a version, and versions must only ever increase.
# This trips if the GOBL requirement is ever moved backwards, in which case the
# fix is to roll forward rather than down.
top="$(git tag -l 'v*' | grep -E "$RELEASE_RE" | sort -V | tail -n1 || true)"
if [[ -n "$top" && "$(printf '%s\n%s\n' "$top" "$next" | sort -V | tail -n1)" != "$next" ]]; then
	echo "next-version: refusing $next, not above current highest $top (GOBL requirement downgraded?)" >&2
	exit 1
fi

echo "$next"
