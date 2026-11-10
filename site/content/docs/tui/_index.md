---
title: TUI
weight: 6
---

`drg tui` opens a full screen dashboard of a Project. The dashboard is made of tabs, and each tab arranges tiles. A tile shows one view, like the list of [Tasks]({{< relref "/docs/concepts/tasks" >}}). The [views page]({{< relref "/docs/tui/views" >}}) lists every view with its options and its keys. Run it from a project directory, the one you ran [`drg project init`]({{< relref "/docs/cli/project/init" >}}) in:

```sh
drg tui
```

Outside a project directory it fails with the same error the other task commands give.

Each `task-list` tile shows the task list with the same rows as [`drg task list`]({{< relref "/docs/cli/task/list" >}}), plus the parent of each task. When the tile is too narrow for every column, the parent column drops first, then the ticket, the blockers and the ID. The status and the title always stay, and a title that does not fit is cut with `…`.

Each `drudger-list` tile shows the [Drudgers]({{< relref "/docs/concepts/drudgers" >}}) of the project with their slot, what DRUDGE last saw of the sandbox and of the agent, the task each one works on and how long ago DRUDGE last looked at it. When the tile is too narrow, the last check drops first, then the task, the agent and the sandbox. The slot always stays.

A `task-detail` tile shows the fields of one task and its description rendered as markdown. Moving the cursor of a list onto a task selects it, and every `task-detail` tile of the same tab shows it. Other tabs keep their own selection. Until you select a task, the tile says `Nothing selected`. When the tile is too short for the fields and three lines of the description, the fields fold into one line. `j`, `k`, `u`, `d`, `g` and `G` scroll the description.

Press `enter` on a task to open it. Focus moves to the first `task-detail` tile of the tab, which shows the task. A tab without a `task-detail` tile opens the task in a box over the tab. The box takes every key until `esc` closes it and focus goes back to the tile that had it. While the box is open, the bottom bar shows only its status segment.

Press `/` to filter the focused list by title, in any case. While the filter is open, every key goes to it, `q`, `1` to `9`, `[` and `]` included, `↑` and `↓` move the cursor and `ctrl+c` still quits. `esc` closes the filter and shows every task again.

`drg tui` reloads its data every 2 seconds. Set `dataRefreshSeconds` in the [TUI config]({{< relref "/docs/configuration/tui" >}}) to change that, or press `r` to reload right away. A load that fails leaves the last data on screen and shows the error in the bottom bar.

A reload reads the Drudgers as DRUDGE last recorded them and asks no sandbox. Every 30 seconds `drg tui` also checks the Drudgers the way [`drg drudger list`]({{< relref "/docs/cli/drudger/list" >}}) does, in the background. Set `drudgerCheckSeconds` to change that, or press `R` to check right away. A check never starts while the last one runs.

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
              { "view": "drudger-list" }
            ]
          },
          {
            "arrangement": "side-by-side",
            "children": [
              { "view": "task-list" },
              { "view": "task-detail" }
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

The left side of the bottom bar shows a status segment for each tile of the tab, in layout order. A `task-list` tile shows the row of its cursor and its task count, like `12/200`. While its filter is open, the filter comes first, like `/retry 1/3`. A `drudger-list` tile says how long ago the Drudgers were checked, like `checked 12s ago`. A `task-detail` tile shows the short ID of its task and how far the description is scrolled, like `3f9a1c2e 40%`. Set `showStatus` to `false` on a tile to hide its segment. Load errors and warnings follow the segments. `? help` sits on the right and is hidden while a filter is open. When the bar is too narrow, the segments are cut first.

One tile of a tab has focus. Its title starts with `▸` and the keys you press go to it. Each tab keeps its own focused tile when you switch tabs. `z` zooms the focused tile to fill the tab and adds `[zoom]` to its title. Press `z` again to go back to the tiles. While a tile is zoomed, the bottom bar shows only the status segment of that tile.

When the terminal is too small to show every tile of a tab, the tab shows a warning in place of its tiles. Make the terminal bigger or zoom a tile with `z`.

`?` opens a box over the tabs that lists the keys of the focused tile and the keys of the whole dashboard. While the box is open, every other key does nothing and `ctrl+c` still quits. `esc` or `?` closes it. A terminal smaller than the box shows the part of it that fits.

| Key | Does |
| --- | --- |
| `j` or `↓` | Moves the cursor down, or scrolls a list without a cursor |
| `k` or `↑` | Moves the cursor up, or scrolls a list without a cursor |
| `g` or `home` | Jumps to the top |
| `G` or `end` | Jumps to the bottom |
| `enter` | Opens the task under the cursor |
| `/` | Opens the filter of the focused list |
| `esc` | Closes the filter and clears it |
| `r` | Reloads the data |
| `R` | Checks the Drudgers |
| `1` to `9` | Goes to that tab |
| `[` or `]` | Goes to the previous or next tab |
| `ctrl+h`, `ctrl+j`, `ctrl+k`, `ctrl+l` or `ctrl+` arrows | Moves focus to the tile left, below, above or right |
| `tab` or `shift+tab` | Moves focus to the next or previous tile in layout order |
| `z` | Zooms the focused tile or restores the tiles |
| `?` | Lists every key, `esc` or `?` closes the list |
| `q` or `ctrl+c` | Quits |

`drg tui` captures the mouse. A click on a tab in the top bar goes to that tab. A click on a tile gives it focus, and a click on a row moves the cursor there. The wheel scrolls the tile under the pointer, focused or not. The mouse does nothing while the help or the task overlay is open. Tiles take their sizes from the config and never resize by dragging. While the mouse is captured, most terminals select text with `shift`+drag. Set `captureMouse` to `false` in the [TUI config]({{< relref "/docs/configuration/tui" >}}) to keep the native text selection of the terminal.

Colors come from the [theme]({{< relref "/docs/configuration/theme" >}}). An override in the theme file that is not a color shows as a warning in the bottom bar.
