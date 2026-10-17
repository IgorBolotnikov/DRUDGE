---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task pr
linkTitle: pr
weight: 10
generated: true
---

```text
usage: drg task pr <task-id>
```

Open the pull requests whose descriptions are still in the run directory of a task.

It retries the ones that failed at close-out, and opens them for a task that finished before pull requests were turned on.

Each branch is pushed to origin from the repository in the project directory, whatever task the Drudger holds now.

A task an agent is still working on is refused.

The task ID may be the short one a listing prints, as long as it names a single task.

## Arguments

- `<task-id>`
