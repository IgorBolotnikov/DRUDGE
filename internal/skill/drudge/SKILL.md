---
name: DRUDGE
description: Create, change, list, show and remove DRUDGE tasks with the drg CLI. Load it before you write a spec, plan or ticket into DRUDGE tasks, or before you change or remove existing ones.
---

# Writing DRUDGE tasks with `drg`

`drg` stores the tasks of the project linked to the current directory. A drudger picks up a task and works from its description alone, so the description is the whole prompt the agent gets.

This skill covers `drg task new`, `edit`, `list`, `show` and `rm`. Run `drg task <subcommand> --help` for the exact flags of each.

## Words

- A "ticket ID" names the external work item, like `ABC-123`. Every task split out of it carries that ID.
- "Tasks" are the pieces a spec is split into.

## Rules

1. Before you write anything, run `drg task list --ticket <ticket-id>`. If tasks already exist, show them beside the new breakdown. Propose leave, edit, create or remove for each one and wait for the user to approve.
2. Never edit or remove a task that is `in-progress` or `done`. Report it to the user and leave it alone.
3. Write every description with a quoted heredoc into `--description-file -`. The quotes around the heredoc delimiter keep the shell from expanding `$`, backticks and quotes.
4. Name a task by its short id, the first 8 characters of its full id. Every command and every flag that takes a task id accepts it, as long as it names a single task.
5. Create tasks one at a time in dependency order. `drg task new` prints `Created task [<id>] <title>`. Take the short id from that line and pass it to `--blocked-by` on the later tasks that wait for it. Use `--parent` when a task belongs under another task.
6. Put `--ticket <ticket-id>` on every task.
7. Never pass `--status` to `drg task new`. The config picks the status of a new task. Never promote a task to another status unless the user asks.
8. If a call fails halfway through, fix the cause and carry on from the task that failed. The tasks created before it already exist, so do not create them again.

## Commands

### Create a task

```sh
drg task new --title "Parse task new arguments in one function" --ticket R-006 --description-file - <<'EOF'
Move the flag parsing of `taskNew` into `parseTaskNewArgs`.

Every error message stays word for word.
EOF
```

It prints `Created task [3f1c9a2e-5b7d-4e8a-9c1f-2d6b8e4a7c05] Parse task new arguments in one function`. A later task that waits for it names its short id:

```sh
drg task new --title "Read a description from stdin" --ticket R-006 --blocked-by 3f1c9a2e --description-file - <<'EOF'
...
EOF
```

### Change a task

```sh
drg task edit 3f1c9a2e --title "Parse task new arguments in parseTaskNewArgs" --description-file - <<'EOF'
...the whole new description...
EOF
```

A new description replaces the old one completely. `--blocked-by` replaces the whole list of blockers. `--block` and `--unblock` add and remove single ones.

### List tasks

```sh
drg task list --ticket R-006
```

The listing prints the short id of each task.

### Show a task

```sh
drg task show 3f1c9a2e
```

It prints the description and the status. Read it before you edit a task.

### Remove a task

```sh
drg task rm 3f1c9a2e --force
```

`drg task rm` asks for confirmation on the terminal. Pass `--force` only for a task the user approved to remove. Tasks blocked by it are unblocked, and tasks under it are ungrouped.
