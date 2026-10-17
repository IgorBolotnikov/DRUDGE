---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg project delete
linkTitle: delete
weight: 2
generated: true
---

```text
usage: drg project delete <name> [options]
```

Delete a project and everything drudge keeps for it in the drudge home directory, its tasks included. It asks first.

The project may be named by its name or its slug.

## Arguments

- `<name>`

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-f, --force` | Delete the project without asking | `false` |
