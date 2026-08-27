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

  createdAt
  updatedAt
  deletedAt
```
