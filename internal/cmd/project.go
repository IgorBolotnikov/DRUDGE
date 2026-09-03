package cmd

import (
	"fmt"
	"path/filepath"

	"drudge/internal/adapters/persistence"
	"drudge/internal/common"
	"drudge/internal/project"
)

var ProjectCmd = &Cmd{
	Name:  "project",
	Usage: "project <subcommand>",
	Desc:  "Project management commands",
	Run:   runProject,
}

func runProject(args []string) error {
	if len(args) < 1 {
		return ErrNoProjectName
	}

	switch args[0] {
	case "create":
		return projectCreate(args[1:])
	case "init":
		return projectInit(args[1:])
	case "delete":
		return projectDelete(args[1:])
	case "rename":
		return projectRename(args[1:])
	case "list":
		return projectList()
	default:
		return ErrNoProjectName
	}
}

func projectCreate(args []string) error {
	if len(args) < 1 {
		return ErrNoProjectName
	}

	name := args[0]

	log := common.NewLogger("")
	repo := persistence.NewFileProjectRepository("")
	svc := project.NewProjectService(repo, log)

	_, err := svc.CreateProject(name)
	return err
}

type projectConfig struct {
	ProjectSlug string `json:"projectSlug"`
}

func projectInit(args []string) error {
	if len(args) < 1 {
		return ErrNoProjectName
	}

	name := args[0]

	log := common.NewLogger("")
	repo := persistence.NewFileProjectRepository("")
	svc := project.NewProjectService(repo, log)

	proj, err := svc.CreateProject(name)
	if err != nil {
		return err
	}

	localDrudgeDir := common.DotDrudgeDirName
	localConfigPath := filepath.Join(localDrudgeDir, common.LocalConfigName)

	if err := common.EnsureDir(localDrudgeDir); err != nil {
		return fmt.Errorf("could not create %s directory: %w", localDrudgeDir, err)
	}

	cfg := projectConfig{ProjectSlug: proj.Slug}
	if err := common.WriteJSON(localConfigPath, cfg); err != nil {
		return fmt.Errorf("could not write config: %w", err)
	}

	log.Info("Initialized project %s in %s", name, localDrudgeDir)
	return nil
}

func projectDelete(args []string) error {
	if len(args) < 1 {
		return ErrNoProjectName
	}

	lookup := args[0]

	repo := persistence.NewFileProjectRepository("")
	log := common.NewLogger("")
	svc := project.NewProjectService(repo, log)

	proj, err := svc.LookupProject(lookup)
	if err != nil {
		return fmt.Errorf("could not find project %q: %w", lookup, err)
	}

	name := proj.Name

	force := HasForceFlag(args)

	if err := ConfirmDeletion(fmt.Sprintf("project %q", name), force); err != nil {
		return err
	}

	if err := repo.DeleteProject(proj.Slug); err != nil {
		return fmt.Errorf("could not delete project %q: %w", name, err)
	}

	fmt.Printf("Removed project %q\n", name)
	return nil
}

func projectRename(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: drg project rename <old-name> <new-name>")
	}

	oldName := args[0]
	newName := args[1]

	repo := persistence.NewFileProjectRepository("")
	log := common.NewLogger("")
	svc := project.NewProjectService(repo, log)

	return svc.RenameProject(oldName, newName)
}

func projectList() error {
	log := common.NewLogger("")
	repo := persistence.NewFileProjectRepository("")
	svc := project.NewProjectService(repo, log)

	projects, err := svc.ListProjects()
	if err != nil {
		return fmt.Errorf("could not list projects: %w", err)
	}

	if len(projects) == 0 {
		log.Info("No projects found")
		return nil
	}

	log.Info("Projects (%d):", len(projects))
	log.Info("  %-20s  %s", "slug", "name")
	log.Info("  --------------------  --------")
	for _, p := range projects {
		log.Info("  %-20s  %s", p.Slug, p.Name)
	}

	return nil
}
