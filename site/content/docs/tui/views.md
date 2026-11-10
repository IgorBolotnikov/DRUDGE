---
# Generated from the View registry. Do not edit it, regenerate it with go test ./internal/ui/tui -run TestViewsPage -update
title: Views
weight: 1
generated: true
---

A tile shows the view its `view` key names in the [TUI config]({{< relref "/docs/configuration/tui" >}}). `viewOptions` of the tile sets the options of the view. Press `?` in `drg tui` to list the keys of the focused tile.

## `drudger-list`

Lists the Drudgers of the project with what DRUDGE last saw of each.

### View options

The view takes no options.

### Keys

| Key | Does |
| --- | --- |
| `j/k` | scroll |
| `g/G` | top/bottom |

## `task-list`

Lists the tasks of the project with the rows of drg task list.

### View options

| Option | Type | Default | Allowed | Description |
| --- | --- | --- | --- | --- |
| `status` | string |  | `draft`, `todo`, `in-progress`, `fucked-up`, `unmerged`, `done` | Shows only the tasks with this status, without grouping them under their parent. Without it the list shows every task. |

### Keys

| Key | Does |
| --- | --- |
| `j/k` | move |
| `g/G` | top/bottom |
| `/` | filter |
| `esc` | clear filter |
