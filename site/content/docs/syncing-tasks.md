---
title: Syncing Tasks
weight: 5
---

The [Tasks]({{< relref "/docs/concepts/tasks" >}}) of a Project live in the DRUDGE home directory of one machine. To carry them to another device, export them to a zip archive and copy the archive over.

## Export

```sh
drg task export
```

Run it from a project directory, or pass `-p <slug>` to export any Project from anywhere. `-p` wins over the project directory you are in.

The archive holds every task file of the Project, whatever its status, byte for byte as it is on disk, plus a `manifest.json` with the format version, the slug of the Project and the export time. It goes to `<slug>-<count>-tasks-<YYYYMMDD-HHMMSS>.zip` in the current directory, with the time in UTC. Pass `-o <path>` to write it somewhere else.

Export refuses to:

- overwrite a file that already exists at the archive path.
- export a Project with no tasks.
- write anything when one task file does not parse. The error names the file and what is wrong with it, so fix the file and run the export again.

The [CLI reference]({{< relref "/docs/cli/task/export" >}}) lists every option.
