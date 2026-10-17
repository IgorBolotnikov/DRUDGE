---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task runlog
linkTitle: runlog
weight: 12
generated: true
---

```text
usage: drg task runlog <task-id>
```

Print the event stream and the stderr log of the latest run of a task. Lines that are JSON are indented.

The task ID may be the short one a listing prints, as long as it names a single task.

## Arguments

- `<task-id>`
