package project

import (
	"fmt"
	"path/filepath"
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

func (p *ProjectService) CreateProject(name string) error {
	if name == "" {
		return fmt.Errorf("project name is required")
	}

	slug := common.SlugFrom(name)
	home, err := common.HomeDir()
	if err != nil {
		return err
	}
	location := filepath.Join(common.ProjectsDir(home), slug)

	dto := CreateProjectDto{
		Name:      name,
		Slug:      slug,
		Location:  location,
		CreatedAt: time.Now(),
	}

	_, err = p.repo.CreateProject(dto)
	if err != nil {
		return fmt.Errorf("could not create project %q: %w", name, err)
	}

	p.log.Info("Created project %s", name)
	return nil
}

func (p *ProjectService) ListProjects() ([]*Project, error) {
	return p.repo.ListProjects()
}

func (p *ProjectService) LookupProject(slugOrName string) (*Project, error) {
	return p.repo.LookupProject(slugOrName)
}

func (p *ProjectService) RenameProject(oldName string, newName string) error {
	project, err := p.repo.LookupProject(oldName)
	if err != nil {
		return fmt.Errorf("project %q not found: %w", oldName, err)
	}

	return p.repo.RenameProject(project.Slug, newName)
}
