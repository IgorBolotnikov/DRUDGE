---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task done
linkTitle: done
weight: 6
generated: true
---

```text
usage: drg task done <task-id>
```

Mark an unmerged task done once its work is merged, and list the tasks that became runnable.

A task in any other status is refused. Use drg task edit --status done to mark it done anyway.

The task ID may be the short one a listing prints, as long as it names a single task.

## Arguments

- `<task-id>`
