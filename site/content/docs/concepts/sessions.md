---
title: Sessions and runs
weight: 4
---

A Session is a single run of work on a Task by one Drudger. A Drudger hosts at most one Session at a time. A Task can be worked on many times, so it can have many Sessions over its life. Only the last one is tracked.

## Starting a Session

[`drg task run`]({{< relref "/docs/cli/task/run" >}}) claims a [Drudger]({{< relref "drudgers" >}}), puts its workspace on the branch of the Task, starts the agent in its sandbox and waits for the first output of the agent. Then it returns, and the agent works on its own. `--dry-run` prints the prompt the agent would get and starts nothing.

The prompt is the description of the Task wrapped in a template. A prompt file named in the [config]({{< relref "/docs/configuration" >}}) swaps in your own template.

The agent writes a Session id on its first event. The agent harness resumes a Session by that id, and DRUDGE records it on the Task.

## The run directory

Every run writes its files to `.drudge/runs/<task-id>/` in the project directory:

- `prompt.txt`: the prompt the agent got
- `harness`: the agent harness that launched the run. `drg task runlog`, close-out and the Session checks read the stream with it, so a change to the configured harness leaves old runs readable. A run with no `harness` file is read with the configured harness. Close-out and the Session checks fail on a harness DRUDGE cannot read the stream of, and so does `drg task run` before it claims a Drudger
- `setup.log`: the output of the [setup commands]({{< relref "/docs/configuration/project-config#setup" >}}) of the repositories, written when the project has any
- `stream.jsonl`: the event stream of the agent, one JSON event per line
- `stderr.log`: what the agent wrote to stderr
- `exit`: the exit code of the agent, written once it exits
- `pull-requests/`: the pull request descriptions [close-out]({{< relref "close-out" >}}) has not opened a pull request from yet

The sandbox mounts the runs directory, so the agent writes its stream where DRUDGE reads it. A new run of the same Task deletes the directory and starts it again with only the new prompt and harness. [`drg task rm`]({{< relref "/docs/cli/task/rm" >}}) deletes it with the Task.

[`drg task runlog`]({{< relref "/docs/cli/task/runlog" >}}) prints the event stream and the stderr log. [`drg task show`]({{< relref "/docs/cli/task/show" >}}) prints what the last run left on the Task.

## How it is going

[`drg task status`]({{< relref "/docs/cli/task/status" >}}) reads the run directory and says how the Session is going:

- **working**: the agent runs and keeps writing.
- **needs babysitting**: the agent runs and has written nothing for 5 minutes.
- **fucked up**: the agent stopped without finishing the work.
- **got shit done**: the agent finished the work it was given.
- **never got going**: the vendor turned the agent away.

Once the Session has ended, `drg task status` records the outcome on the Task: what the agent said last, how many turns it took, how long it worked and what it cost. Close-out runs at that point, and the Task ends up `done`, `unmerged` or `fucked-up`. See [Tasks]({{< relref "tasks" >}}) for what the statuses mean.

## When the vendor turns the agent away

A vendor can refuse an agent before it does any work: a rate limit, an outage or an expired login. A Session that the vendor turned away never got going. The agent was refused before it could work, so the Task is not at fault and goes back to `todo`. The Task records what the vendor said and which kind of refusal it was. The Drudger goes idle and records its agent as refused.

Fix whatever the vendor named, like logging in again for an expired login, and run the Task again.
