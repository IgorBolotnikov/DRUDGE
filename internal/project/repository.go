package project

import "time"

type CreateProjectDto struct {
	Name     string
	Slug     string
	Location string
	CreatedAt time.Time
}

type ProjectRepository interface {
	CreateProject(dto CreateProjectDto) (*Project, error)
	ListProjects() ([]*Project, error)
	LookupProject(slug string) (*Project, error)
	RenameProject(slug string, newName string) error
	DeleteProject(slug string) error
}
