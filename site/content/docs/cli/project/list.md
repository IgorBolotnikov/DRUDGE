---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg project list
linkTitle: list
weight: 5
generated: true
---

```text
usage: drg project list [options]
```

List the projects drudge knows about, with the slug and the name of each, one page at a time.

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-p, --page <number>` | The number of the page to show | `1` |
| `--page-size <items>` | How many items one page shows, 0 shows them all, project.pageSize from the global config when left out |  |
