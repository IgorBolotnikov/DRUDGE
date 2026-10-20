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

A task whose id the project already has is skipped. Tasks of another project are moved into this one.

It reads the whole archive first and stops at an archive it cannot read, before it writes anything.

## Arguments

- `<file>`

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-p, --project <slug>` | The slug of the project to import into, the project of the current directory when left out |  |
