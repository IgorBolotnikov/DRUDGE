---
title: Tasks
weight: 2
---

A Task is a piece of work to be done. Its description is the prompt the agent gets, so write it the way you would brief someone who has never seen the code.

## One file per Task

Every Task is one markdown file under `~/.drudge/projects/<slug>/tasks/`, named after its id and its title. The file starts with a front matter block of flat `key: value` lines between `---` lines. The description follows as the raw markdown body.

The front matter holds what DRUDGE knows about the Task: its status, its ticket, its parent, what it waits for, the repositories it changes, the pull requests opened for it and the record of its last [Session]({{< relref "sessions" >}}). It does not say which [Drudger]({{< relref "drudgers" >}}) works on it. The Drudger knows that.

Every write re-reads the file under a lock, so two commands never overwrite each other's fields. Change a Task with [`drg task edit`]({{< relref "/docs/cli/task/edit" >}}) and DRUDGE keeps it consistent.

Add a Task with [`drg task new`]({{< relref "/docs/cli/task/new" >}}), read it with [`drg task show`]({{< relref "/docs/cli/task/show" >}}), list them with [`drg task list`]({{< relref "/docs/cli/task/list" >}}) and delete one with [`drg task rm`]({{< relref "/docs/cli/task/rm" >}}). A command takes the short id a listing prints, as long as it names one Task.

## Statuses

A status says where a Task stands.

- `draft`: written and waiting for you to review it.
- `todo`: ready to run. Only a `todo` Task can be handed to a Drudger. A new Task is `todo` unless you say otherwise or set `task.defaultStatus` in the [config]({{< relref "/docs/configuration" >}}).
- `in-progress`: handed to a Drudger.
- `fucked-up`: the Session ended without getting the work done, or its Drudger was nuked while it worked.
- `unmerged`: the Session got shit done and its commits are on a branch that is not merged yet.
- `done`: the work is finished. A Session that got shit done and left no commits ends here. So does an `unmerged` Task you mark done with [`drg task done`]({{< relref "/docs/cli/task/done" >}}) once its work is merged.

You move a Task between `draft`, `todo` and `done`. DRUDGE sets `in-progress`, `fucked-up` and `unmerged` itself, and `drg task edit` sets them only when forced. A Task the vendor turned away goes back to `todo`, see [Sessions and runs]({{< relref "sessions" >}}).

[`drg task rerun`]({{< relref "/docs/cli/task/rerun" >}}) starts an `in-progress`, `fucked-up` or `unmerged` Task over from scratch.

## Dependencies

A Task can be blocked by other Tasks. A blocked Task is refused by `drg task run` and `drg task rerun` until every Task it waits for is `done`. When a Task becomes `done`, DRUDGE lists the Tasks that became runnable.

[`drg task next`]({{< relref "/docs/cli/task/next" >}}) prints the oldest `todo` Task with no unfinished blocker. It starts nothing.

Deleting a Task unblocks the Tasks that waited for it.

## Repositories

A Task can list the repositories of the [project]({{< relref "projects" >}}) it changes. A Task that lists none changes all of them. The agent sees every repository of the project and is told to leave the unlisted ones alone.

## Tickets and parents

A Task can carry the id of the ticket it came from, in whatever tracker you use. DRUDGE puts it in the prompt and can put it in the [branch name]({{< relref "close-out" >}}). `drg task list` filters by it.

A Task can belong to a parent Task. Grouping is one level deep: a parent cannot belong to another Task, and a Task with children cannot get a parent. `drg task list` shows the children under their parent. Deleting a parent ungroups its children.

A parent does not block its children, and its children do not block it. Dependencies set the order.
