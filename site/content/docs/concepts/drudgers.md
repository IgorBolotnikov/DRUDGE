---
title: Drudgers
weight: 3
---

A Drudger is a slot, a sandbox and a workspace. The agent of a Drudger runs in its sandbox and works in its workspace. The three are made together, stay together and are reused from one Task to the next. Nothing is rebuilt between Tasks.

## Slot, sandbox and workspace

The slot is the number of the Drudger in the pool of its [project]({{< relref "projects" >}}), starting at 1.

The workspace is `.drudge/worktrees/slot-<n>/` in the project directory. It holds a git worktree per repository of the project, at the same path the repository has in the project directory. The worktrees share the git history of your own checkouts, so a commit an agent makes is in your repository right away, on its own branch.

The sandbox is a Docker sandbox made with `sbx` over the workspace. It mounts the workspace, the `.git` directory of every repository and the run directory of the project.

The pool lives in `~/.drudge/projects/<slug>/drudgers.json`. Each entry records the slot, the sandbox, the harness the sandbox was made for, the workspace, the Task occupying it and what DRUDGE last saw of the sandbox, the workspace and the agent. [`drg drudger list`]({{< relref "/docs/cli/drudger/list" >}}) shows it.

## Switching the harness

A sandbox is made for one harness, the `drudger.harness` of the [global config]({{< relref "/docs/configuration/config" >}}) at the time the Drudger is made. An entry without a harness was made for `claude-code`. After you switch `drudger.harness`, `drg task run` refuses to launch onto a Drudger made for the old harness and names it. Nuke each such Drudger with [`drg drudger nuke <slot>`]({{< relref "/docs/cli/drudger/nuke" >}}). The next launch makes a new Drudger for the configured harness in that slot.

## Occupied or idle

A Drudger is either occupied with a Task or idle. [`drg task run`]({{< relref "/docs/cli/task/run" >}}) takes the lowest free slot. A slot with no Drudger yet gets one, and an idle Drudger is reused. Before the agent starts, the workspace is checked, whatever the last [Session]({{< relref "sessions" >}}) left uncommitted is stashed, and every repository the Task changes is put on the branch of the Task.

A Drudger goes idle when its Session ends, whether that Session got shit done or fucked up.

## How many work in parallel

`maxConcurrentDrudgers` caps how many Drudgers work on one project at once. It is set in the [global config]({{< relref "/docs/configuration/config" >}}) and can be overridden in the [project config]({{< relref "/docs/configuration/project-config" >}}). When every slot under the cap is occupied, `drg task run` refuses the Task. A Drudger above a lowered cap stays in the pool and gets no new Task.

## Health

The sandbox, the workspace and the agent break independently, so a Drudger has one state for each of them. A perfectly good sandbox can hold an agent the vendor turns away. `drg drudger list` shows what DRUDGE last saw of each, and when.

A Drudger answers whether it is able to work. How the work itself is going is a question for the Session.

## Reclaiming

A Drudger frees its slot when its agent writes an exit file at the end of the Session. An agent killed before that leaves the Drudger occupied by a Task with nothing working on it, and that slot counts against the cap.

[`drg drudger reclaim`]({{< relref "/docs/cli/drudger/reclaim" >}}) puts such a Drudger back to idle. It frees a slot whose launch never made a run directory, and one whose sandbox is not running. It kills no process and removes no sandbox. The Task keeps its status, so start it over with [`drg task rerun`]({{< relref "/docs/cli/task/rerun" >}}).

## Parking

Parking takes a Drudger off the branches of its last Task. Whatever the Session left uncommitted is stashed, and every worktree is detached back to the base its repository cuts work from. Git refuses to delete a branch a worktree has checked out, so parking is what lets you delete or check out a Task branch in your own checkout.

[Close-out]({{< relref "close-out" >}}) parks the workspace when a Session ends, and `drg drudger reclaim` parks every idle Drudger. The stashes stay in the repository, and close-out records the ones it makes on the Task.

## Nuking

[`drg drudger nuke <slot>`]({{< relref "/docs/cli/drudger/nuke" >}}) deletes a Drudger. It stashes what the worktrees hold uncommitted, removes them, deletes the sandbox and drops the slot from the pool. The branches stay. The next `drg task run` that needs the slot builds a new Drudger in it.

A Drudger with a running Session is refused unless you force it. Forcing kills the agent along with the sandbox, and its Task ends up `fucked-up`.
