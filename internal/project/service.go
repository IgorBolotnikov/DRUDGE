package project

import (
	"fmt"
	"time"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/git"
)

type ProjectService struct {
	repo   ProjectRepository
	gitOps git.Operations
	log    *common.Logger
}

func NewProjectService(repo ProjectRepository, gitOps git.Operations, log *common.Logger) *ProjectService {
	return &ProjectService{repo: repo, gitOps: gitOps, log: log}
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

	p.log.Info("Created project %s", name)
	return proj, nil
}

// InitProject creates a project for projectDir and links the current directory
// to it in the local config file, together with the repositories of
// projectDir. A directory holding no repository is refused before the project
// is created.
func (p *ProjectService) InitProject(name string, projectDir string) (*Project, []config.Repository, error) {
	repositories, err := p.DiscoverRepositories(projectDir)
	if err != nil {
		return nil, nil, err
	}

	created, err := p.CreateProject(name)
	if err != nil {
		return nil, nil, err
	}

	localCfg := config.LocalConfig{ProjectSlug: created.Slug, Repositories: repositories}
	if err := localCfg.Save(); err != nil {
		return nil, nil, err
	}
	return created, repositories, nil
}

func (p *ProjectService) ListProjects() ([]*Project, error) {
	return p.repo.ListProjects()
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

	if err := p.repo.RenameProject(found.Slug, newName); err != nil {
		return fmt.Errorf("could not rename project %s: %w", found.Slug, err)
	}
	p.log.Info("Renamed project %s from %q to %q", found.Slug, found.Name, newName)
	return nil
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
