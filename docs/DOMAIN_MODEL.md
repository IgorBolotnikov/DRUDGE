# Domain Model

The whole drudge lives inside `~/.drudge/`

## Projects

Location: `~/.drudge/projects/*`

```
model Project
  slug
  name
  location (abs path)

  createdAt
```

## Tasks

Location: `~/.drudge/projects/<project>/tasks`

```
model Task
  id
  slug
  title
  // Desctiption contains the whole task body
  // We don't enforce that content
  description
  status
  // Full ids of the tasks this task waits for, comma-separated
  // Stored under the blocked_by key and left out when empty
  blockedBy
  // Full id of the task this task belongs to. Grouping is one level deep
  // Stored under the parent_task_id key and left out when empty
  parentTaskId

  createdAt
  updatedAt
  deletedAt
```
