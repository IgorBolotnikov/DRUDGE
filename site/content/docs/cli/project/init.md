---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg project init
linkTitle: init
weight: 1
generated: true
---

```text
usage: drg project init <name>
```

Record a project and link the current directory to it. Task commands run here work on that project.

A directory that is a git repository becomes the one repository of the project.

Otherwise every subdirectory that is a git repository becomes one, and a directory holding none is refused.

It prints each repository with the default branch that task branches are cut from.

## Arguments

- `<name>`
