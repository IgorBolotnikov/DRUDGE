---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task import
linkTitle: import
weight: 14
generated: true
---

```text
usage: drg task import <file> [options]
```

Add every task of an archive drg task export wrote that the project does not have yet, with its id, its timestamps and its run state as they are.

A task whose id the project already has is replaced when the archived copy has a newer update time, and skipped otherwise. A task in progress here is always skipped. Tasks of another project are moved into this one.

It reads the whole archive first and stops at an archive it cannot read, before it writes anything.

An entry that is not a task file it can import is skipped with a warning. The other tasks are imported and the command fails at the end.

## Arguments

- `<file>`

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-p, --project <slug>` | The slug of the project to import into, the project of the current directory when left out |  |
