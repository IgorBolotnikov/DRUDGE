// Package persistence provides persistence adapters
package persistence

import (
	"fmt"
	"os"
	"path/filepath"

	"drudge/internal/common"
	"drudge/internal/project"
)

const ProjectConfigFile = "project.json"

type FileProjectRepository struct {
	root string
}

func NewFileProjectRepository(root string) *FileProjectRepository {
	return &FileProjectRepository{root: root}
}

func (r *FileProjectRepository) CreateProject(dto project.CreateProjectDto) (*project.Project, error) {
	projectDir := filepath.Join(r.root, dto.Slug)
	if err := common.EnsureDir(projectDir); err != nil {
		return nil, fmt.Errorf("could not create directory %s: %w", projectDir, err)
	}

	proj := &project.Project{
		Slug:      dto.Slug,
		Name:      dto.Name,
		Location:  dto.Location,
		CreatedAt: dto.CreatedAt,
	}

	projectFile := filepath.Join(projectDir, ProjectConfigFile)
	if err := common.WriteJSON(projectFile, proj); err != nil {
		return nil, fmt.Errorf("could not write project file %s: %w", projectFile, err)
	}

	return proj, nil
}

func (r *FileProjectRepository) DeleteProject(slug string) error {
	projectDir := filepath.Join(r.root, slug)

	exists, err := common.Exists(projectDir)
	if err != nil {
		return err
	}
	if !exists {
		fmt.Printf("Nothing to delete, %q does not exist\n", slug)
		return nil
	}

	if err := common.RemoveAll(projectDir); err != nil {
		return fmt.Errorf("could not remove project %q: %w", slug, err)
	}

	return nil
}

func (r *FileProjectRepository) LookupProject(slugOrName string) (*project.Project, error) {
	slug := common.SlugFrom(slugOrName)

	entries, err := os.ReadDir(r.root)
	if err != nil {
		return nil, fmt.Errorf("could not list projects: %w", err)
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}

		projFile := filepath.Join(r.root, e.Name(), ProjectConfigFile)
		_, err := os.Stat(projFile)
		if err != nil {
			continue
		}

		var proj project.Project
		if err := common.ReadJSON(projFile, &proj); err != nil {
			continue
		}

		if e.Name() == slug || common.SlugFrom(proj.Name) == slug {
			return &proj, nil
		}
	}

	return nil, fmt.Errorf("project %q not found", slugOrName)
}

func (r *FileProjectRepository) RenameProject(slug string, newName string) error {
	newSlug := common.SlugFrom(newName)
	oldDir := filepath.Join(r.root, slug)
	newDir := filepath.Join(r.root, newSlug)

	if slug != newSlug {
		if exists, err := common.Exists(filepath.Join(r.root, newSlug)); err != nil {
			return err
		} else if exists {
			return fmt.Errorf("project with slug %q already exists, cannot rename", newSlug)
		}
	}

	projFile := filepath.Join(oldDir, ProjectConfigFile)

	// Read current project data
	var proj project.Project
	if err := common.ReadJSON(projFile, &proj); err != nil {
		return fmt.Errorf("could not read project.json: %w", err)
	}

	proj.Name = newName
	proj.Slug = newSlug
	proj.Location = newDir

	// Write updated project.json
	if err := common.WriteJSON(projFile, proj); err != nil {
		return fmt.Errorf("could not write project.json: %w", err)
	}

	// Move directory if slug changed
	if slug != newSlug {
		if err := os.Rename(oldDir, newDir); err != nil {
			// Rollback: restore old slug in project.json
			proj.Slug = slug
			proj.Location = oldDir
			common.WriteJSON(projFile, proj)
			return fmt.Errorf("could not rename directory %s -> %s: %w", oldDir, newDir, err)
		}
	}

	return nil
}

func (r *FileProjectRepository) ListProjects() ([]*project.Project, error) {
	entries, err := os.ReadDir(r.root)
	if err != nil {
		return nil, fmt.Errorf("could not list projects: %w", err)
	}

	var projects []*project.Project
	for _, d := range entries {
		if !d.IsDir() {
			continue
		}

		projFile := filepath.Join(r.root, d.Name(), ProjectConfigFile)
		_, err := os.Stat(projFile)
		if err != nil {
			continue
		}

		var proj project.Project
		if err := common.ReadJSON(projFile, &proj); err != nil {
			continue
		}

		projects = append(projects, &proj)
	}

	return projects, nil
}
