---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg drudger list
linkTitle: list
weight: 1
generated: true
---

```text
usage: drg drudger list [options]
```

List the Drudgers of the current project and what each one is doing, one page at a time.

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-p, --page <number>` | The number of the page to show | `1` |
| `--page-size <items>` | How many items one page shows, 0 shows them all, drudger.pageSize from the project config or the global config when left out |  |
