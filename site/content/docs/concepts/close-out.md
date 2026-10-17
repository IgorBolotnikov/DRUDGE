---
title: Close-out
weight: 5
---

Close-out is what DRUDGE does with the work of a Session once the Session ends. It finds the commits the agent made, opens their pull requests and [parks]({{< relref "drudgers#parking" >}}) the workspace. It runs when [`drg task status`]({{< relref "/docs/cli/task/status" >}}) records the outcome of the Session.

## Branches

Before the agent starts, every repository the Task changes is put on one branch, cut from the default branch of that repository. A change spanning two repositories sits on the same branch name in both. Repositories the Task does not list stay on the commit their worktree sits on.

`task.branchFormat` in the [config]({{< relref "/docs/configuration" >}}) names the branch. The default carries the short id and the title of the Task. Put the `{{ticketID}}` placeholder in the format to name branches after the ticket of the Task.

A rerun that finds the branch of the earlier attempt holding commits takes the next name in the series, with `-2`, `-3` and so on appended. Every attempt that committed something keeps its own branch.

## What close-out records

Close-out reads where the agent left each repository and records on the Task the branch the work is on, the commit it was cut from, the commit it ends at and how many commits it holds.

- A repository the agent committed nothing to loses its branch.
- An agent that switched to another branch has its work recorded on that branch.
- Commits the agent left on a detached HEAD get a branch.
- Commits in a repository the Task does not list are recorded and reported. Their pull request is not opened.

A Session that got shit done leaves the Task `unmerged` when it has commits in a repository it lists, and `done` when it has none. Merge the branches and mark the Task done with [`drg task done`]({{< relref "/docs/cli/task/done" >}}).

## Pull requests

Pull requests are off by default. Turn them on with the `remote` keys of the [config]({{< relref "/docs/configuration" >}}). GitHub is the one provider so far, and DRUDGE opens pull requests through the `gh` CLI.

With pull requests on, the prompt tells the agent to write a pull request description to `.drudge/pull-request.md` at the root of every repository it committed to, without committing it. The first line is the title and the rest is the body, filled in from the pull request template of the repository or from a default one.

When a Session got shit done, close-out moves each description into the [run directory]({{< relref "sessions#the-run-directory" >}}), pushes the branch to `origin` and opens a pull request into the default branch. The URL goes on the Task and the description is deleted. A repository with commits and no description gets no pull request.

A description whose pull request failed stays in the run directory. So does the description of a repository the Task does not list.

## drg task pr

[`drg task pr`]({{< relref "/docs/cli/task/pr" >}}) opens the pull requests whose descriptions are still in the run directory of a Task. Use it to retry the ones that failed, to open the one for a repository the Task did not list, or to open them for a Task that finished before you turned pull requests on.

It pushes each branch from the repository in the project directory, so it works whatever Task the Drudger holds by then. It refuses a Task an agent is still working on.
