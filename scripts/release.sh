#!/usr/bin/env bash
# Tags the next version and pushes the tag, which kicks off the release workflow.
#
# Usage: scripts/release.sh feat|fix [message]
#
# feat bumps the minor version, fix bumps the patch version. Without a message
# git opens an editor for the tag annotation.
set -euo pipefail

kind="${1:-}"
message="${2:-}"

die() {
	echo "release: $*" >&2
	exit 1
}

case "$kind" in
feat | fix) ;;
*) die "usage: make release feat|fix [MSG=\"...\"]" ;;
esac

[ "$(git rev-parse --abbrev-ref HEAD)" = "main" ] || die "not on main"
[ -z "$(git status --porcelain)" ] || die "working tree is dirty"

git fetch --quiet --tags origin main
git merge-base --is-ancestor HEAD origin/main || die "HEAD is not pushed to origin/main"

latest="$(git tag --list 'v*.*.*' --sort=-v:refname | head -n1)"
latest="${latest:-v0.0.0}"

if [ -n "$(git tag --points-at HEAD --list 'v*')" ]; then
	die "HEAD is already tagged as $(git tag --points-at HEAD --list 'v*' | head -n1)"
fi

IFS=. read -r major minor patch <<<"${latest#v}"
case "$kind" in
feat) next="v${major}.$((minor + 1)).0" ;;
fix) next="v${major}.${minor}.$((patch + 1))" ;;
esac

echo "release: $latest -> $next"

if [ -n "$message" ]; then
	git tag -a "$next" -m "$message"
else
	git tag -a "$next"
fi

git push origin "$next"
