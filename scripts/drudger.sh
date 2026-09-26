#!/usr/bin/env bash
# Reviews and merges the branch a Drudger left in its worktree.
#
# Usage: scripts/drudger.sh review|merge [slot]
#
# review rebases the Drudger branch onto main and opens it in tuicr.
# merge runs the tests in the worktree and fast-forwards main to the branch.
# The slot can be left out when the project has a single Drudger worktree.
set -euo pipefail

action="${1:-}"
slot="${2:-}"

die() {
	echo "drudger: $*" >&2
	exit 1
}

case "$action" in
review | merge) ;;
*) die "usage: make review|merge [slot]" ;;
esac

root="$(git rev-parse --show-toplevel)"
worktrees_dir="$root/.drudge/worktrees"

if [ -z "$slot" ]; then
	slots=("$worktrees_dir"/slot-*)
	[ -d "${slots[0]}" ] || die "no Drudger worktrees in $worktrees_dir"
	[ "${#slots[@]}" -eq 1 ] || die "more than one Drudger worktree, pass a slot: make $action <slot>"
	slot="${slots[0]##*/slot-}"
fi

worktree="$(git worktree list --porcelain | sed -n "s|^worktree \($worktrees_dir/slot-$slot\(/.*\)\?\)$|\1|p" | head -n1)"
[ -n "$worktree" ] || die "slot $slot has no worktree for this repository"

branch="$(git -C "$worktree" symbolic-ref --quiet --short HEAD)" || die "worktree of slot $slot has no branch checked out"
[ -z "$(git -C "$worktree" status --porcelain)" ] || die "worktree of slot $slot has uncommitted changes"

slug="$(jq -r '.projectSlug' "$root/.drudge/config.json")"
task="$(jq -r --argjson slot "$slot" '.drudgers[] | select(.slot == $slot) | .task // empty' "$HOME/.drudge/projects/$slug/drudgers.json")"
[ -z "$task" ] || die "slot $slot is working on task $task"

[ -n "$(git rev-list main.."$branch")" ] || die "$branch has no commits on top of main"

case "$action" in
review)
	git -C "$worktree" rebase main || die "rebase of $branch stopped on a conflict, resolve it in $worktree"
	git log --oneline main.."$branch"
	tuicr -r "main..$branch"
	;;
merge)
	[ "$(git rev-parse --abbrev-ref HEAD)" = "main" ] || die "not on main"
	[ -z "$(git status --porcelain)" ] || die "working tree is dirty"
	git merge-base --is-ancestor main "$branch" || die "$branch is not on top of main, run make review $slot first"
	(cd "$worktree" && go test ./...) || die "tests fail on $branch"
	git merge --ff-only "$branch"
	;;
esac
