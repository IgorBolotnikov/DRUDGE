---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg project rename
linkTitle: rename
weight: 3
generated: true
---

```text
usage: drg project rename <old-name> <new-name>
```

Give a project a new name. Its slug and its files stay, so directories linked to it keep working.

The project may be named by its name or its slug. A name another project goes by is refused.

## Arguments

- `<old-name>`
- `<new-name>`
