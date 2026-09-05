- Introduce tasks into the DRUDGE. These will be stored in markdown files as a starting point.

- Add a task list command `drg task list` which lists all tasks in current project (project slug taken from .drudge/config.json in current dir). If current dir does not have a project, then show the message about that

- New command `drg task show [task ID]` which prints task to console

- Move `filterTasks` and `sortTasksDesc` out of `internal/cmd/task.go` and into `TaskService`. They're business logic sitting in the CLI layer right now, and the TUI/web interfaces will need the same filtering and sorting.

- Add shell autocomplete for the CLI (bash, zsh, fish). Completing Drudger names by hand is the worst of it — `drudge-claude-<slug>-<n>` is long, easy to typo, and there is no way to discover the ones that exist without running another command first. Task ids have the same problem from the other end: a listing prints eight characters and completion should offer exactly those, alongside titles so you can tell which is which. Subcommands, flags and project slugs are the easy wins. Needs the CLI to be able to enumerate its own commands, which the hand-rolled dispatch in `internal/cmd` cannot do today.

- Make the Drudgers file write crash-safe. `writeDrudgersFile` truncates the file in place, so a process that dies mid-write leaves it empty or half-written and every allocation after that fails to parse it. The pool wedges, which is the failure the lock was meant to rule out. The lock stops two processes interleaving and does nothing about one dying. Fix is to write a temp file and rename over it, which is atomic. The lock already lives in its own file, so the inode changing underneath it breaks nothing.
