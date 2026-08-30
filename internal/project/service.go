package project

import (
	"fmt"
	"time"

	"drudge/internal/common"
)

type ProjectService struct {
	repo ProjectRepository
	log  *common.Logger
}

func NewProjectService(repo ProjectRepository, log *common.Logger) *ProjectService {
	return &ProjectService{repo: repo, log: log}
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
