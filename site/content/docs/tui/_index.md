---
title: TUI
weight: 6
---

`drg tui` opens a full screen dashboard of a Project. The dashboard is made of tabs, and each tab arranges tiles. A tile shows one view, like the list of [Tasks]({{< relref "/docs/concepts/tasks" >}}). Run it from a project directory, the one you ran [`drg project init`]({{< relref "/docs/cli/project/init" >}}) in:

```sh
drg tui
```

Outside a project directory it fails with the same error the other task commands give.

Each `task-list` tile shows the task list with the same rows as [`drg task list`]({{< relref "/docs/cli/task/list" >}}). It loads the tasks once when it starts. Restart it to see changes.

It sets the title of the terminal window to `DRUDGE | <project name>`. Terminals that don't support window titles ignore it.

The tabs and the tiles of each tab come from the [TUI config]({{< relref "/docs/configuration/tui" >}}). Without a TUI config file `drg tui` shows this:

```json
{
  "$schema": "./schema/tui.json",
  "tabs": [
    {
      "title": "overview",
      "layout": {
        "arrangement": "stacked",
        "children": [
          {
            "arrangement": "side-by-side",
            "children": [
              { "view": "task-list", "weight": 2 },
              { "view": "task-list" }
            ]
          },
          {
            "arrangement": "side-by-side",
            "children": [
              { "view": "task-list" },
              { "view": "task-list" }
            ]
          }
        ]
      }
    },
    {
      "title": "tasks",
      "layout": { "view": "task-list" }
    }
  ]
}
```

When the terminal is too small to show every tile of a tab, the tab shows a warning in place of its tiles. Make the terminal bigger to see them.

| Key | Does |
| --- | --- |
| `j` or `↓` | Moves the cursor down |
| `k` or `↑` | Moves the cursor up |
| `g` or `home` | Jumps to the top |
| `G` or `end` | Jumps to the bottom |
| `1` to `9` | Goes to that tab |
| `[` or `]` | Goes to the previous or next tab |
| `q` or `ctrl+c` | Quits |

Colors come from the [theme]({{< relref "/docs/configuration/theme" >}}). An override in the theme file that is not a color shows as a warning in the bottom bar.
