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

Detect the setup commands and copy paths of every repository of the project from the files in its root, and write them to the setup and copy keys of each repository in the local config.

A version file like .tool-versions, mise.toml, .nvmrc or .python-version gets mise and the pinned toolchains installed.

The first lockfile found gets its packages installed. A repository that matches nothing gets an empty list.

A .pre-commit-config.yaml gets pre-commit and its hooks installed into the worktree only, through a hooks directory of the sandbox. Hooks a package manager installs, like husky, get no lines.

Files git ignores whose name starts with .env, like .env and .env.local, and a .envs directory get copied. A .envrc is never copied.

Every ${NAME} in a .npmrc is added to drudger.env of the local config, for every repository. Names already there stay and no name is ever removed, --force included.

It reads only the root of each repository.

A repository that has a setup key keeps it and its copy key, an empty list included, unless --force is given.

A repository with no setup key keeps a copy key it has, unless --force is given.

## Options

| Option | Description | Default |
| --- | --- | --- |
| `--dry-run` | Print the detected setup and copy paths and write nothing | `false` |
| `-f, --force` | Replace the setup and copy paths of every repository, existing ones included | `false` |
