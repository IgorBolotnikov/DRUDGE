---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg task export
linkTitle: export
weight: 13
generated: true
---

```text
usage: drg task export [options]
```

Write every task file of a project, whatever its status, into a new zip archive along with a manifest.

It reads every task file first and stops at the first one that does not parse, before it writes anything.

A project with no tasks and an archive path that already exists are refused.

## Options

| Option | Description | Default |
| --- | --- | --- |
| `-o, --output <path>` | Write the archive to path, \<slug>-\<count>-tasks-\<YYYYMMDD-HHMMSS>.zip in the current directory when left out |  |
| `-p, --project <slug>` | The slug of the project to export, the project of the current directory when left out |  |
