---
# Generated from the drg command tree. Do not edit it, regenerate it with go test ./internal/cmd -run TestCLIReference -update
title: drg project detect
linkTitle: detect
weight: 2
generated: true
---

```text
usage: drg project detect [options]
```

Detect the setup commands of every repository of the project from the files in its root, and write them to the setup key of each repository in the local config.

A version file like .tool-versions, mise.toml, .nvmrc or .python-version gets mise and the pinned toolchains installed.

The first lockfile found gets its packages installed. A repository that matches nothing gets an empty list.

It reads only the root of each repository.

A repository that has a setup key keeps it, an empty list included, unless --force is given.

## Options

| Option | Description | Default |
| --- | --- | --- |
| `--dry-run` | Print the detected setup and write nothing | `false` |
| `-f, --force` | Replace the setup of every repository, existing ones included | `false` |
