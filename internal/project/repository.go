package project

import "time"

type CreateProjectDto struct {
	Name      string
	Slug      string
	Location  string
	CreatedAt time.Time
}

type ProjectRepository interface {
	CreateProject(dto CreateProjectDto) (*Project, error)
	ListProjects() ([]*Project, error)
	RenameProject(slug string, newName string) error
	DeleteProject(slug string) error
}

// DirectoryLinker links the current directory to the project with the slug
// slug and records the repositories of the project.
type DirectoryLinker interface {
	LinkDirectory(slug string, repositories []Repository) error
}
