---
title: Syncing Tasks
weight: 5
---

The [Tasks]({{< relref "/docs/concepts/tasks" >}}) of a Project live in the DRUDGE home directory of one machine. To carry them to another device, export them to a zip archive, copy the archive over and import it there.

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

## Import

Copy the archive to the other device and import it into a Project there:

```sh
drg task import <slug>-<count>-tasks-<YYYYMMDD-HHMMSS>.zip
```

Import resolves the Project the same way export does. The Project has to exist already, so run `drg project init` first on a fresh device.

Every task in the archive whose ID the Project does not have yet is added as it is. It keeps its ID, its creation and update times, its status and its run state. A task that is `in-progress` comes in as `in-progress`, and import notes that it is in progress on the other device. A blocker or a parent the archive does not hold stays as a reference to a task the Project does not have.

A task whose ID the Project already has is compared with the copy there:

- A task that is `in-progress` in the Project is skipped, even when the archive holds a newer copy. A Drudger on this device holds it.
- An archived copy with a newer update time replaces the task as it is, with the timestamps of the archived copy. A changed title renames the task file.
- An archived copy with the same or an older update time is skipped.

Importing the same archive twice changes nothing the second time. An import in the other direction right after a sync changes nothing either.

When the archive comes from a Project with another slug, import moves every task into the target Project and warns once, naming both slugs.

Import prints one line per task, `✓` for an added or updated task and `·` for a skipped one with the reason, and ends with how many tasks it added, updated, skipped as older and skipped as in progress.

Import writes nothing when the archive is not a zip, holds no `manifest.json`, holds a manifest or a task file that does not parse, or has a format version this `drg` does not read. Make a fresh export on the source device and import that.

## Limits

- Deletes don't sync. Remove a task on both devices.
- A task edited on both devices between syncs keeps only the newer edit. Sync before you switch devices.
- The comparison trusts the clocks of both devices. A device with a clock that runs ahead wins over newer edits made on the other one.

The CLI reference lists every option of [export]({{< relref "/docs/cli/task/export" >}}) and [import]({{< relref "/docs/cli/task/import" >}}).
