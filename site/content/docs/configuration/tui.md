---
title: TUI
weight: 4
---

The TUI config file is `~/.drudge/tui.json`. It sets the tabs of [`drg tui`]({{< relref "/docs/tui" >}}) and the layout of each tab. DRUDGE never writes it. Without the file `drg tui` shows the bundled default, which the [TUI page]({{< relref "/docs/tui" >}}) shows as JSON. Copy it into the file to start from it.

`drg tui` reads the file when it starts. A file that does not parse, or that holds an unknown key, a layout DRUDGE cannot build or a view option the view refuses, stops `drg tui` with an error that names the file and the path of the value to change, like `tabs[0].layout.children[1].weight`.

## Schema

Start the file with `"$schema": "./schema/tui.json"`, a path relative to the file. It points at `~/.drudge/schema/tui.json`, the local copy of the schema that `drg setup` rewrites on every run. An editor with JSON schema support uses it to complete keys and flag wrong values.

## Layout

A layout is a tree of nodes. A node is a tile or a split.

- A tile sets `view`, the thing it shows, and may set `title`, `showStatus` and `viewOptions`. `viewOptions` holds the options of the view, like `{"status": "todo"}` for a `task-list`. The [views page]({{< relref "/docs/tui/views" >}}) lists the options of each view.
- A split sets `arrangement` and two or more `children`. `side-by-side` places the children left to right and `stacked` places them top to bottom.

A split divides its space between its children by `weight`. A child with weight 2 gets twice the space of a child with weight 1.

## Keys

<!-- BEGIN GENERATED: edits between these markers are overwritten -->

| Key | Type | Default | Allowed | Description |
| --- | --- | --- | --- | --- |
| `$schema` | string |  |  | JSON schema reference for configuration validation |
| `tabs` | array |  |  | The tabs in the order of the top bar. Keys 1 to 9 go to the first nine. |
| `tabs[].title` | string |  |  | Required. The title of the tab in the top bar. Each tab has its own title. |
| `tabs[].layout` | object |  |  | Required. The node that fills the tab: a tile or a split. |
| `tabs[].layout.arrangement` | string |  | `"side-by-side"`, `"stacked"` | How the split places its children. |
| `tabs[].layout.children` | array |  |  | The nodes of the split. Each child has the keys of a layout. |
| `tabs[].layout.view` | string |  | `"drudger-list"`, `"task-list"` | What the tile shows. `drudger-list`: Lists the Drudgers of the project with what DRUDGE last saw of each. `task-list`: Lists the tasks of the project with the rows of drg task list. |
| `tabs[].layout.title` | string |  |  | The title of the tile in its top border. Only a tile has a title. |
| `tabs[].layout.weight` | number | `1` | above 0 | The share of the space of its split the node takes, relative to the other children of the split. It may be a fraction. |
| `tabs[].layout.showStatus` | boolean | `true` |  | Whether the tile shows its status segment on the left of the bottom bar. Only a tile has a status segment. |
| `tabs[].layout.viewOptions` | object |  |  | The options of the view of the tile. Each view takes its own options. Only a tile has view options. |
| `dataRefreshSeconds` | number | `2` | minimum 0.1 | How often drg tui reloads its data, in seconds. It is the only key in DRUDGE ending in Seconds that takes a fraction, like 0.5. |
| `drudgerCheckSeconds` | integer | `30` | minimum 1 | How often drg tui checks the Drudgers with what drg drudger list does, in whole seconds. A check never starts while the last one runs. |
| `captureMouse` | boolean | `true` |  | Whether drg tui captures the mouse. A click focuses a tile, switches tabs or moves the cursor, and the wheel scrolls the tile under the pointer. When it is false, the terminal keeps its native text selection. |

<!-- END GENERATED -->
