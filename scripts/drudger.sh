#!/usr/bin/env bash
# Reviews and merges the branch a Drudger left for a task.
#
# Usage: scripts/drudger.sh review|merge [task-id]
#
# The task ID can be shortened to any unique prefix. It can be left out when a
# single task has a branch with commits on top of main.
#
# The branch is checked out in a review worktree of its own, so the Drudgers
# keep their worktrees and can take new tasks during a review.
#
# review rebases the branch onto main in the review worktree and opens it in
# tuicr.
# merge runs the tests in the review worktree, fast-forwards main to the branch
# and removes the review worktree and the branch.
set -euo pipefail

action="${1:-}"
task_prefix="${2:-}"

status_in_progress="in-progress"

die() {
	echo "drudger: $*" >&2
	exit 1
}

case "$action" in
review | merge) ;;
*) die "usage: make review|merge [task-id]" ;;
esac

root="$(git rev-parse --show-toplevel)"
repository="$(basename "$root")"
slug="$(jq -r '.projectSlug' "$root/.drudge/config.json")"
tasks_dir="$HOME/.drudge/projects/$slug/tasks"
review_dir="$root/.drudge/review"

# front_matter prints the value of a key from the front matter of a task file.
front_matter() {
	awk -v key="$2" '
		NR == 1 && $0 == "---" { inside = 1; next }
		inside && $0 == "---" { exit }
		inside && index($0, key ": ") == 1 { print substr($0, length(key) + 3) }
	' "$1"
}

branch_of() {
	front_matter "$1" "branch.$repository"
}

has_commits_on_main() {
	git show-ref --verify --quiet "refs/heads/$1" && [ -n "$(git rev-list main.."$1")" ]
}

# pending_task_files prints every task file whose branch has commits on top of
# main.
pending_task_files() {
	local file branch
	for file in "$tasks_dir"/*.md; do
		branch="$(branch_of "$file")"
		if [ -n "$branch" ] && has_commits_on_main "$branch"; then
			echo "$file"
		fi
	done
}

if [ -n "$task_prefix" ]; then
	task_files=("$tasks_dir/$task_prefix"*.md)
	[ -e "${task_files[0]}" ] || die "no task ID starts with $task_prefix in project $slug"
	[ "${#task_files[@]}" -eq 1 ] || die "more than one task ID starts with $task_prefix, give more of it"
	task_file="${task_files[0]}"
else
	mapfile -t task_files < <(pending_task_files)
	[ "${#task_files[@]}" -gt 0 ] || die "no task of project $slug has a branch with commits on top of main"
	if [ "${#task_files[@]}" -gt 1 ]; then
		echo "drudger: more than one task has a branch to review, pass a task ID: make $action <task-id>" >&2
		for file in "${task_files[@]}"; do
			echo "  $(basename "$file" .md)" >&2
		done
		exit 1
	fi
	task_file="${task_files[0]}"
fi

task_id="$(front_matter "$task_file" id)"
task_status="$(front_matter "$task_file" status)"
branch="$(branch_of "$task_file")"

[ "$task_status" != "$status_in_progress" ] || die "task $task_id is still running"
[ -n "$branch" ] || die "task $task_id records no branch for repository $repository"
git show-ref --verify --quiet "refs/heads/$branch" || die "branch $branch of task $task_id does not exist"
[ -n "$(git rev-list main.."$branch")" ] || die "$branch has no commits on top of main"

worktree="$review_dir/${task_id:0:8}"

# checkout_path prints the worktree that has a branch checked out.
checkout_path() {
	git worktree list --porcelain | awk -v ref="refs/heads/$1" '
		/^worktree / { path = substr($0, 10) }
		$0 == "branch " ref { print path }
	'
}

if [ -d "$worktree" ]; then
	[ "$(git -C "$worktree" symbolic-ref --quiet --short HEAD)" = "$branch" ] ||
		die "review worktree $worktree is off $branch, finish what is going on there or remove it with: git worktree remove $worktree"
	[ -z "$(git -C "$worktree" status --porcelain)" ] || die "review worktree $worktree has uncommitted changes"
else
	holder="$(checkout_path "$branch")"
	[ -z "$holder" ] || die "$branch is checked out in $holder, detach it with: git -C $holder checkout --detach"
	git worktree add --quiet "$worktree" "$branch"
fi

case "$action" in
review)
	git -C "$worktree" rebase main || die "rebase of $branch stopped on a conflict, resolve it in $worktree"
	git log --oneline main.."$branch"
	tuicr -r "main..$branch"
	;;
merge)
	[ "$(git rev-parse --abbrev-ref HEAD)" = "main" ] || die "not on main"
	[ -z "$(git status --porcelain)" ] || die "working tree is dirty"
	git merge-base --is-ancestor main "$branch" || die "$branch is not on top of main, run make review ${task_id:0:8} first"
	(cd "$worktree" && go test ./...) || die "tests fail on $branch"
	git merge --ff-only "$branch"
	git worktree remove "$worktree"
	git branch --delete "$branch"
	;;
esac
