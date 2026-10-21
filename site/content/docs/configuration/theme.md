---
title: Theme
weight: 3
---

The theme file is `~/.drudge/theme.json`. `drg setup` writes it when it is missing, with the theme it asks you for and no overrides. `drg setup -y` and input that is not a terminal take the `system` theme. After that the file is yours to edit.

When the file exists, `drg setup` asks again with the theme of the file preselected and writes the answer back into it. `overrides`, `$schema` and every other key stay as they are. `drg setup -y` and input that is not a terminal leave an existing file alone. A file that does not load, like broken JSON or an unknown theme name, stops `drg setup` with the error before any question is asked.

## Schema

The file starts with `"$schema": "./schema/theme.json"`, a path relative to the file. It points at `~/.drudge/schema/theme.json`, the local copy of the schema that `drg setup` rewrites on every run. An editor with JSON schema support uses it to complete keys and flag wrong values.

## Palettes

`theme` names one of the bundled palettes:

- `system` uses the basic ANSI colors of your terminal and dim. It is the default.
- `nord`, `monokai`, `catppuccin-mocha` and `dracula` use 24-bit colors.

A missing file or a missing `theme` key means `system`. An unknown theme name makes DRUDGE fall back to the default theme and print a warning.

## Overrides

`overrides` maps a role to a `#rrggbb` color that replaces the color of the palette for that role. Every role you leave out keeps the color of the palette.

```json
{
  "$schema": "./schema/theme.json",
  "theme": "nord",
  "overrides": {
    "error": "#FF5555"
  }
}
```

An override that is not a `#rrggbb` color is skipped. Every command prints a warning naming the role and the value, and uses the color of the palette for that role.

## Keys

<!-- BEGIN GENERATED: edits between these markers are overwritten -->

| Key | Type | Default | Allowed | Description |
| --- | --- | --- | --- | --- |
| `$schema` | string |  |  | JSON schema reference for configuration validation |
| `theme` | string | `"system"` | `"system"`, `"nord"`, `"monokai"`, `"catppuccin-mocha"`, `"dracula"` | The color theme to use. |
| `overrides` | object |  |  | Optional per-role color overrides. Each value must be a 24-bit hex color in "#rrggbb" format. Unknown roles are silently ignored. |
| `overrides.primary` | string |  | matches `^#[0-9a-fA-F]{6}$` | Default text / main content color. |
| `overrides.heading` | string |  | matches `^#[0-9a-fA-F]{6}$` | Heading text color. |
| `overrides.success` | string |  | matches `^#[0-9a-fA-F]{6}$` | Success message color. |
| `overrides.error` | string |  | matches `^#[0-9a-fA-F]{6}$` | Error message color. |
| `overrides.warning` | string |  | matches `^#[0-9a-fA-F]{6}$` | Warning message color. |
| `overrides.info` | string |  | matches `^#[0-9a-fA-F]{6}$` | Informational message color. |
| `overrides.muted` | string |  | matches `^#[0-9a-fA-F]{6}$` | Muted / dimmed text color. |
| `overrides.secondary` | string |  | matches `^#[0-9a-fA-F]{6}$` | Secondary text color. |
| `overrides.border` | string |  | matches `^#[0-9a-fA-F]{6}$` | Border / separator color. |
| `overrides.path` | string |  | matches `^#[0-9a-fA-F]{6}$` | File path display color. |
| `overrides.<key>` | string |  | matches `^#[0-9a-fA-F]{6}$` |  |

<!-- END GENERATED -->
