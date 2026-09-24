// Package persistence provides persistence adapters
package persistence

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

const ProjectConfigFile = "project.json"

type FileProjectRepository struct {
	projectsDirPath string
}

func NewFileProjectRepository(projectsDirPath string) *FileProjectRepository {
	return &FileProjectRepository{projectsDirPath: projectsDirPath}
}

func (r *FileProjectRepository) CreateProject(dto project.CreateProjectDto) (*project.Project, error) {
	projectDir, err := r.resolveProjectDir(dto.Slug)
	if err != nil {
		return nil, err
	}
	if err := common.EnsureDir(projectDir); err != nil {
		return nil, fmt.Errorf("could not create directory %s: %w", projectDir, err)
	}

	proj := &project.Project{
		Slug:      dto.Slug,
		Name:      dto.Name,
		Location:  dto.Location,
		CreatedAt: dto.CreatedAt,
	}

	projFile, err := r.resolveProjectFile(dto.Slug)
	if err != nil {
		return nil, err
	}
	err = r.saveProject(projFile, proj)
	if err != nil {
		return nil, fmt.Errorf("could not write project file %s: %w", projFile, err)
	}

	return proj, nil
}

func (r *FileProjectRepository) DeleteProject(slug string) error {
	projectDir, err := r.resolveProjectDir(slug)
	if err != nil {
		return err
	}

	isPresent, err := common.Exists(projectDir)
	if err != nil {
		return err
	}
	if !isPresent {
		fmt.Printf("Nothing to delete, %q does not exist\n", slug)
		return nil
	}

	if err := common.RemoveAll(projectDir); err != nil {
		return fmt.Errorf("could not remove project %q: %w", slug, err)
	}

	return nil
}

// RenameProject writes a new name into the project file of a project. The
// slug and the project directory stay, because the local config of every
// linked directory names the project by its slug.
func (r *FileProjectRepository) RenameProject(slug string, newName string) error {
	projFile, err := r.resolveProjectFile(slug)
	if err != nil {
		return err
	}

	proj, err := r.readProjectFile(slug)
	if err != nil {
		return fmt.Errorf("could not read project file: %w", err)
	}

	proj.Name = newName
	if err := r.saveProject(projFile, proj); err != nil {
		return fmt.Errorf("could not write project file: %w", err)
	}
	return nil
}

// ListProjects reads every project in the projects directory. A projects
// directory that does not exist yet holds no projects.
func (r *FileProjectRepository) ListProjects() ([]*project.Project, error) {
	entries, err := r.readProjectEntries()
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not list projects: %w", err)
	}

	var projects []*project.Project
	for _, d := range entries {
		if !d.IsDir() {
			continue
		}
		proj, err := r.readProjectFile(d.Name())
		if err != nil {
			continue
		}

		projects = append(projects, proj)
	}

	return projects, nil
}

func (r *FileProjectRepository) readProjectEntries() ([]os.DirEntry, error) {
	projDir, err := r.resolveProjectsDir()
	if err != nil {
		return []os.DirEntry{}, err
	}

	return os.ReadDir(projDir)
}

func (r *FileProjectRepository) readProjectFile(slug string) (*project.Project, error) {
	projFile, err := r.resolveProjectFile(slug)
	if err != nil {
		return nil, err
	}
	_, err = os.Stat(projFile)
	if err != nil {
		return nil, err
	}

	var proj project.Project
	if err := common.ReadJSON(projFile, &proj); err != nil {
		return nil, err
	}
	return &proj, nil
}

func (r *FileProjectRepository) resolveProjectsDir() (string, error) {
	if r.projectsDirPath != "" {
		return r.projectsDirPath, nil
	}
	home, err := common.HomeDir()
	if err != nil {
		return "", err
	}
	return common.ProjectsDir(home), nil
}

func (r *FileProjectRepository) resolveProjectDir(slug string) (string, error) {
	projectsDir, err := r.resolveProjectsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(projectsDir, slug), nil
}

func (r *FileProjectRepository) saveProject(path string, proj *project.Project) error {
	return common.WriteJSON(path, proj)
}

func (r *FileProjectRepository) resolveProjectFile(slug string) (string, error) {
	projDir, err := r.resolveProjectsDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(projDir, slug, ProjectConfigFile), nil
}
