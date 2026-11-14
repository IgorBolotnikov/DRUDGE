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

Each `task-board` tile shows the tasks on a board with one column per status, in the order of `draft`, `todo`, `in-progress`, `fucked-up`, `unmerged` and `done`. The header of a column shows its status and its task count. The header of the column under the cursor is drawn in reverse, so the cursor stays visible in an empty column. `h` and `l` or `←` and `→` move between columns, and a title that does not fit is cut with `…`. When the columns would get narrower than 15 cells of title, the board turns compact: every status shows as a header with its count, and only the column under the cursor lists its tasks. A task with a status DRUDGE does not know shows in no column.

A `task-detail` tile shows the fields of one task and its description rendered as markdown. Moving the cursor of a list or a board onto a task selects it, and every `task-detail` tile of the same tab shows it. Other tabs keep their own selection. Until you select a task, the tile says `Nothing selected`. The values of the fields start at the same column for every task. A blank line separates the fields from the description. When the tile is too short for the fields, the blank line and three lines of the description, the fields fold into one line with no blank line below. `j`, `k`, `u`, `d`, `g`, `G` and the wheel scroll the description.

Press `enter` on a task to open it. Every `task-detail` tile of the tab shows the task, and focus moves to the first one. A tab without a `task-detail` tile opens the task in a box over the tab. The box takes every key but `?` until `esc` closes it and focus goes back to the tile that had it. The box shows its status in its bottom border and the bottom bar shows `esc close` and `? help` as the hint.

Press `L` on a task of a `task-list` or `task-board` tile, or in a `task-detail` tile or box, to open its runlog in a box over the tab. From a task box the runlog opens over it, and `esc` goes back to the task box. The box shows the latest run of the task the way [`drg task runlog`]({{< relref "/docs/cli/task/runlog" >}}) does and adds what the run writes after the box opened. It opens at the bottom and follows new steps. Scrolling up pauses it, and scrolling back to the bottom or `G` follows again. The bottom border says `following` or `paused`. When the task runs again, the box shows the new run. A task with no run says so until a run starts. A line wider than the box wraps at spaces, and the lines of a tool call wrap under the start of its summary. Resizing the box wraps the lines again and keeps the step at the top in place while the box is paused.

A `task-runlog` tile shows the runlog of the task selected in a list or a board of the same tab, the way a `task-detail` tile shows the task. Selecting another task drops the steps of the last one and shows the run of the new one from the bottom, following it. Until you select a task, the tile says `No task selected`. When a tab has a `task-runlog` tile, `L` shows the task in the tile and focuses it.

Press `/` to filter the focused list by title, in any case. While the filter is open, every key goes to it, `q`, `1` to `9`, `[` and `]` included, `↑` and `↓` move the cursor and `ctrl+c` still quits. `esc` closes the filter and shows every task again.

A tile gets its data from feeds, and runs only the feeds of what it shows. A `task-list`, a `task-board` and a `task-detail` tile list the tasks and check the [Sessions]({{< relref "/docs/concepts/sessions" >}}). A `drudger-list` tile reads the Drudgers as DRUDGE last recorded them, which asks no sandbox, and checks them the way [`drg drudger list`]({{< relref "/docs/cli/drudger/list" >}}) does. A runlog box or tile reads what the run of its task wrote since its last read, and keeps reading after the run finished. The feeds of other tabs pause until you switch back, and a tab you switch back to shows its last data right away. Each feed runs again at its own interval, which `refreshSeconds` in the [TUI config]({{< relref "/docs/configuration/tui" >}}) sets. A run never starts while the last run of its feed is going. A run that fails leaves the last data on screen and shows its error in the bottom bar until a run works or the feed pauses. The Sessions feed records each finished Session on its task the way [`drg task status`]({{< relref "/docs/cli/task/status" >}}) does, so `drg tui` runs [close-out]({{< relref "/docs/concepts/close-out" >}}) and opens pull requests while it is open.

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
              { "view": "task-board" },
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

The bottom border of each tile shows its status on the right. A `task-list` tile shows the row of its cursor and its task count, like `12/200`. While its filter is open, the count is the count of the tasks that match. A `task-board` tile shows the status of its cursor column, the row of its cursor and the task count of the column, like `todo 2/5`. A `drudger-list` tile says how long ago the Drudgers were checked, like `checked 12s ago`. A `task-detail` tile shows the short ID of its task and how far the description is scrolled, like `3f9a1c2e 40%`. A status wider than the border is cut from the start. Set `showStatus` to `false` on a tile to hide its status.

The bottom bar shows the errors of the feeds and warnings on the left and `? help` on the right. `? help` is hidden while a filter is open. When the bar is too narrow, the errors and warnings are cut with `…`.

One tile of a tab has focus. Its title starts with `▸`, its border takes the primary color of the theme and the keys you press go to it. While the help or the task overlay is open, the overlay takes that border and the tile under it draws as unfocused. Each tab keeps its own focused tile when you switch tabs. `z` zooms the focused tile to fill the tab and adds `[zoom]` to its title. Press `z` again to go back to the tiles.

When the terminal is too small to show every tile of a tab, the tab shows a warning in place of its tiles. Make the terminal bigger or zoom a tile with `z`.

`?` opens a box over the tabs that lists the keys in two sections, one above the other. The first section carries the title of the focused tile and lists the keys of its view. The `Global` section below it lists the keys that work everywhere on the dashboard. Over the task box, the first section lists the keys of the task box and `Global` lists `esc` and `?`. While the help is open, every other key does nothing and `ctrl+c` still quits. `esc` or `?` closes it. A terminal smaller than the box shows the part of it that fits.

| Key | Does |
| --- | --- |
| `j` or `↓` | Moves the cursor down, or scrolls a list without a cursor |
| `k` or `↑` | Moves the cursor up, or scrolls a list without a cursor |
| `g` or `home` | Jumps to the top |
| `G` or `end` | Jumps to the bottom |
| `h` or `←` | Moves the cursor of a board to the column on the left |
| `l` or `→` | Moves the cursor of a board to the column on the right |
| `enter` | Opens the task under the cursor |
| `L` | Opens the runlog of the task under the cursor of a list or a board, or of the task shown in a detail |
| `/` | Opens the filter of the focused list |
| `esc` | Closes the filter and clears it, or closes the top box |
| `1` to `9` | Goes to that tab |
| `[` or `]` | Goes to the previous or next tab |
| `ctrl+h`, `ctrl+j`, `ctrl+k`, `ctrl+l` or `ctrl+` arrows | Moves focus to the tile left, below, above or right |
| `tab` or `shift+tab` | Moves focus to the next or previous tile in layout order |
| `z` | Zooms the focused tile or restores the tiles |
| `?` | Lists the keys of the focused tile or of the top box, `esc` or `?` closes the list |
| `q` or `ctrl+c` | Quits |

`drg tui` captures the mouse. A click on a tab in the top bar goes to that tab. A click on a tile gives it focus, and a click on a row moves the cursor there. A click on a column header of a board moves the cursor to that column. The wheel scrolls the tile under the pointer, focused or not. While a box is open, the wheel and clicks go to the top box and the tabs under it get nothing. The wheel scrolls the task box. Tiles take their sizes from the config and never resize by dragging. While the mouse is captured, most terminals select text with `shift`+drag. Set `captureMouse` to `false` in the [TUI config]({{< relref "/docs/configuration/tui" >}}) to keep the native text selection of the terminal.

Colors come from the [theme]({{< relref "/docs/configuration/theme" >}}). An override in the theme file that is not a color shows as a warning in the bottom bar.
