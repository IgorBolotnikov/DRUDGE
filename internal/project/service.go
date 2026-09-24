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

	slug := common.SlugFrom(name)
	if p.projectExists(slug) {
		return nil, fmt.Errorf("project %s already exists", slug)
	}

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

func (p *ProjectService) LookupProject(slugOrName string) (*Project, error) {
	slug := common.SlugFrom(slugOrName)
	return p.repo.LookupProject(slug)
}

func (p *ProjectService) RenameProject(oldName string, newName string) error {
	project, err := p.repo.LookupProject(oldName)
	if err != nil {
		return fmt.Errorf("project %q not found: %w", oldName, err)
	}

	return p.repo.RenameProject(project.Slug, newName)
}

func (p *ProjectService) projectExists(slug string) bool {
	return p.repo.ProjectExists(slug)
}
