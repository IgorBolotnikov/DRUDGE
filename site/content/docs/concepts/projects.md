---
title: Projects
weight: 1
---

A project is a directory on your machine with the git repositories the agents work on. Every Task belongs to one project, and so does every Drudger.

## Linking a directory

[`drg project init <name>`]({{< relref "/docs/cli/project/init" >}}) records a project and links the current directory to it. The project record lives under `~/.drudge/projects/<slug>/`, next to its Tasks. The link is `.drudge/config.json` in the directory you ran it in, the [project config]({{< relref "/docs/configuration/project-config" >}}).

Task and Drudger commands read the project config of the current directory to find out which project they work on. They fail in a directory without one and tell you to run `drg project init`.

Everything DRUDGE makes in the project directory goes under `.drudge/` there: the project config, the workspaces of the [Drudgers]({{< relref "drudgers" >}}) and the files of the [runs]({{< relref "sessions" >}}).

## Repositories

A project records the git repositories it is made of in the project config.

- A directory that is itself a git repository becomes the one repository of the project.
- Otherwise every immediate subdirectory that is a git repository becomes one.
- A directory holding no repository is refused.

Each repository has a default branch. Task branches are cut from it and pull requests go into it. DRUDGE reads it from `origin/HEAD` unless the repository sets it in the project config. A repository with an `origin` cuts work from the remote branch, which DRUDGE fetches before each handover. A failed fetch prints a warning and the work is cut from what the repository last fetched.

The list is yours to edit. `drg project init` writes it once, and the commands read it on every run.

## Managing projects

[`drg project list`]({{< relref "/docs/cli/project/list" >}}) lists the projects, [`drg project rename`]({{< relref "/docs/cli/project/rename" >}}) gives one a new name and [`drg project delete`]({{< relref "/docs/cli/project/delete" >}}) deletes one along with its Tasks. A rename keeps the slug, so linked directories keep working.
