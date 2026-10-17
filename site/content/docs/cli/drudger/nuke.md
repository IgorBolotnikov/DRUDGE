---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg drudger nuke
linkTitle: nuke
weight: 2
generated: true
---

```text
usage: drg drudger nuke <slot> [options]
```

Delete a Drudger's sandbox and drop it from the pool.

A Drudger with a running Session is refused unless you insist.

## Arguments

- `<slot>`

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-f, --force` | Nuke a working Drudger, killing its agent and fucking up its task | `false` |
