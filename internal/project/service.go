package project

import (
	"errors"
	"fmt"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
)

type ProjectService struct {
	repo     ProjectRepository
	linker   DirectoryLinker
	gitOps   git.Operations
	progress common.Progress
}

// ProjectCreated reports a project CreateProject made.
type ProjectCreated struct {
	Project *Project
}

// ProjectRenamed reports a project RenameProject gave a new name.
type ProjectRenamed struct {
	Slug    string
	OldName string
	NewName string
}

func NewProjectService(repo ProjectRepository, linker DirectoryLinker, gitOps git.Operations, progress common.Progress) *ProjectService {
	return &ProjectService{repo: repo, linker: linker, gitOps: gitOps, progress: progress}
}

func (p *ProjectService) CreateProject(name string) (*Project, error) {
	if name == "" {
		return nil, fmt.Errorf("project name is required")
	}

	projects, err := p.repo.ListProjects()
	if err != nil {
		return nil, err
	}
	if err := refuseTakenName(projects, name, ""); err != nil {
		return nil, err
	}

	slug := common.SlugFrom(name)

	location, err := common.ResolveProjectDir(slug)
	if err != nil {
		return nil, err
	}

	dto := CreateProjectDto{
		Name:      name,
		Slug:      slug,
		Location:  location,
		CreatedAt: time.Now().UTC(),
	}

	proj, err := p.repo.CreateProject(dto)
	if err != nil {
		return nil, fmt.Errorf("could not create project %q: %w", name, err)
	}

	p.progress.Report(ProjectCreated{Project: proj})
	return proj, nil
}

// InitProject creates a project for projectDir and links the current directory
// to it through the DirectoryLinker, together with the repositories of
// projectDir. A directory holding no repository is refused before the project
// is created.
func (p *ProjectService) InitProject(name string, projectDir string) (*Project, []Repository, error) {
	repositories, err := p.DiscoverRepositories(projectDir)
	if err != nil {
		return nil, nil, err
	}

	created, err := p.CreateProject(name)
	if err != nil {
		return nil, nil, err
	}

	if err := p.linker.LinkDirectory(created.Slug, repositories); err != nil {
		return nil, nil, err
	}
	return created, repositories, nil
}

// ListProjects returns the page numbered page of the projects, with size
// projects on every page.
func (p *ProjectService) ListProjects(page int, size int) (common.Page[*Project], error) {
	projects, err := p.repo.ListProjects()
	if err != nil {
		return common.Page[*Project]{}, fmt.Errorf("could not list projects: %w", err)
	}

	listed, err := common.Paginate(projects, page, size)
	var notFound *common.PageNotFoundError
	if errors.As(err, &notFound) {
		notFound.Noun = "projects"
	}
	return listed, err
}

// LookupProject finds the project named by its slug or by its name. Names are
// matched in their slug form.
func (p *ProjectService) LookupProject(slugOrName string) (*Project, error) {
	projects, err := p.repo.ListProjects()
	if err != nil {
		return nil, err
	}
	return findProject(projects, slugOrName)
}

// RenameProject gives the project named by its slug or its name a new name.
// The slug stays. It refuses a name another project goes by.
func (p *ProjectService) RenameProject(slugOrName string, newName string) error {
	if newName == "" {
		return fmt.Errorf("new project name is required")
	}

	projects, err := p.repo.ListProjects()
	if err != nil {
		return err
	}
	found, err := findProject(projects, slugOrName)
	if err != nil {
		return err
	}
	if err := refuseTakenName(projects, newName, found.Slug); err != nil {
		return err
	}

	oldName := found.Name
	if err := p.repo.RenameProject(found.Slug, newName); err != nil {
		return fmt.Errorf("could not rename project %s: %w", found.Slug, err)
	}
	p.progress.Report(ProjectRenamed{Slug: found.Slug, OldName: oldName, NewName: newName})
	return nil
}

// DeleteProject removes the project with the slug slug. It does no lookup by
// name, so a caller holding a name resolves it with LookupProject first.
func (p *ProjectService) DeleteProject(slug string) error {
	return p.repo.DeleteProject(slug)
}

func findProject(projects []*Project, slugOrName string) (*Project, error) {
	slug := common.SlugFrom(slugOrName)
	for _, candidate := range projects {
		if candidate.Slug == slug || common.SlugFrom(candidate.Name) == slug {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("project %q not found", slugOrName)
}

// refuseTakenName refuses a name whose slug form is the slug or the slug form
// of the name of another project. This keeps every lookup down to one project.
// exceptSlug is the project being renamed, and is empty for a new one.
func refuseTakenName(projects []*Project, name string, exceptSlug string) error {
	slug := common.SlugFrom(name)
	for _, other := range projects {
		if other.Slug == exceptSlug {
			continue
		}
		if other.Slug == slug {
			return fmt.Errorf("project %s already exists", slug)
		}
		if common.SlugFrom(other.Name) == slug {
			return fmt.Errorf("project %s is already called %q, pick another name", other.Slug, other.Name)
		}
	}
	return nil
}
