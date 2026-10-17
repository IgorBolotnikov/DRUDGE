---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task rerun
linkTitle: rerun
weight: 9
generated: true
---

```text
usage: drg task rerun <task-id> [options]
```

Start a task over from scratch, clearing what its last run left behind.

Only a task an agent has already had can be rerun, so in-progress, fucked-up and unmerged.

## Arguments

- `<task-id>`

## Options

| Option | Description | Default |
| --- | --- | --- |
| `--dry-run` | Print the prompt the agent would get and stop | `false` |
