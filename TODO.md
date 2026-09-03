- Introduce tasks into the DRUDGE. These will be stored in markdown files as a starting point.

- Add a task list command `drg task list` which lists all tasks in current project (project slug taken from .drudge/config.json in current dir). If current dir does not have a project, then show the message about that

- New command `drg task show [task ID]` which prints task to console

- Move `filterTasks` and `sortTasksDesc` out of `internal/cmd/task.go` and into `TaskService`. They're business logic sitting in the CLI layer right now, and the TUI/web interfaces will need the same filtering and sorting.
