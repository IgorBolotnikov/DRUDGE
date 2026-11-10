---
title: TUI
weight: 6
---

`drg tui` opens a full screen dashboard of a Project. The dashboard is made of tabs, and each tab arranges tiles. A tile shows one view, like the list of [Tasks]({{< relref "/docs/concepts/tasks" >}}). Run it from a project directory, the one you ran [`drg project init`]({{< relref "/docs/cli/project/init" >}}) in:

```sh
drg tui
```

Outside a project directory it fails with the same error the other task commands give.

Each `task-list` tile shows the task list with the same rows as [`drg task list`]({{< relref "/docs/cli/task/list" >}}).

`drg tui` reloads its data every 2 seconds. Set `dataRefreshSeconds` in the [TUI config]({{< relref "/docs/configuration/tui" >}}) to change that, or press `r` to reload right away. A load that fails leaves the last data on screen and shows the error in the bottom bar.

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

The left side of the bottom bar shows a status segment for each tile of the tab, in layout order. A `task-list` tile shows the row of its cursor and its task count, like `12/200`. Set `showStatus` to `false` on a tile to hide its segment. Load errors and warnings follow the segments. Key hints sit on the right. When the bar is too narrow, the segments are cut first.

When the terminal is too small to show every tile of a tab, the tab shows a warning in place of its tiles. Make the terminal bigger to see them.

| Key | Does |
| --- | --- |
| `j` or `↓` | Moves the cursor down |
| `k` or `↑` | Moves the cursor up |
| `g` or `home` | Jumps to the top |
| `G` or `end` | Jumps to the bottom |
| `r` | Reloads the data |
| `1` to `9` | Goes to that tab |
| `[` or `]` | Goes to the previous or next tab |
| `q` or `ctrl+c` | Quits |

Colors come from the [theme]({{< relref "/docs/configuration/theme" >}}). An override in the theme file that is not a color shows as a warning in the bottom bar.
