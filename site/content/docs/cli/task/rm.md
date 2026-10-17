---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task rm
linkTitle: rm
weight: 7
generated: true
---

```text
usage: drg task rm <task-id> [options]
```

Delete a task and the run directory of its Sessions. It asks first.

The branches of the task that hold no commits go with it, and the ones holding work stay.

A task an agent is still working on is refused, so kill its Drudger before removing it.

Tasks blocked by it are unblocked and tasks belonging to it are ungrouped. They stay where they are.

The task ID may be the short one a listing prints, as long as it names a single task.

## Arguments

- `<task-id>`

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-f, --force` | Remove the task without asking | `false` |
