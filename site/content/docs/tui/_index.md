---
title: TUI
weight: 6
---

`drg tui` shows the [Tasks]({{< relref "/docs/concepts/tasks" >}}) of a Project full screen. Run it from a project directory, the one you ran [`drg project init`]({{< relref "/docs/cli/project/init" >}}) in:

```sh
drg tui
```

Outside a project directory it fails with the same error the other task commands give.

The screen shows the task list with the same rows as [`drg task list`]({{< relref "/docs/cli/task/list" >}}). It loads the tasks once when it starts. Restart it to see changes.

| Key | Does |
| --- | --- |
| `j` or `↓` | Moves the cursor down |
| `k` or `↑` | Moves the cursor up |
| `g` or `home` | Jumps to the top |
| `G` or `end` | Jumps to the bottom |
| `q` or `ctrl+c` | Quits |

Colors come from the [theme]({{< relref "/docs/configuration/theme" >}}). An override in the theme file that is not a color shows as a warning in the bottom bar.
