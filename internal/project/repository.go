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
	DeleteProject(slug string) (isRemoved bool, err error)
}

// DirectoryLinker links the current directory to a project and reads that
// link back.
type DirectoryLinker interface {
	// LinkDirectory links the current directory to the project with the slug
	// slug and records the repositories of the project.
	LinkDirectory(slug string, repositories []Repository) error
	// LinkedSlug reads the slug of the project the current directory is
	// linked to. isLinked is false outside a project directory.
	LinkedSlug() (slug string, isLinked bool, err error)
}

// SetupRecorder writes setup commands into the repositories of the local
// config. Setups maps the path of a repository to its commands.
type SetupRecorder interface {
	RecordSetup(setups map[string][]string) error
}
