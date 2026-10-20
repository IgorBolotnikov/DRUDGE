package project

import (
	"io/fs"
	"time"
)

type CreateProjectDto struct {
	Name      string
	Slug      string
	CreatedAt time.Time
}

type ProjectRepository interface {
	CreateProject(dto CreateProjectDto) (*Project, error)
	ListProjects() ([]*Project, error)
	RenameProject(slug string, newName string) error
	DeleteProject(slug string) (isRemoved bool, err error)
}

// RepositoryFiles reads the files of a project directory that detection
// looks at. Paths are host paths.
type RepositoryFiles interface {
	ReadDir(dir string) ([]fs.DirEntry, error)
	Stat(path string) (fs.FileInfo, error)
	ReadFile(path string) ([]byte, error)
}

// DirectoryLinker links the current directory to a project and reads that
// link back.
type DirectoryLinker interface {
	// LinkDirectory links the current directory to the project with the slug
	// slug and records the repositories and the Drudger env of the project.
	LinkDirectory(slug string, repositories []Repository, envNames []string) error
	// LinkedSlug reads the slug of the project the current directory is
	// linked to. isLinked is false outside a project directory.
	LinkedSlug() (slug string, isLinked bool, err error)
}

// DetectedSetup is what detection writes into one repository. A nil Copy
// leaves the copy paths of the repository as they are.
type DetectedSetup struct {
	Setup []string
	Copy  []string
}

// SetupRecorder writes detected setups into the repositories of the local
// config. Setups maps the path of a repository to what detection wrote for it.
// EnvNames are added to the Drudger env.
type SetupRecorder interface {
	RecordSetup(setups map[string]DetectedSetup, envNames []string) error
}
