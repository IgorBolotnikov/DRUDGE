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

	home, err := common.HomeDir()
	if err != nil {
		return err
	}

	log := common.NewLogger("")
	repo := persistence.NewFileProjectRepository(common.ProjectsDir(home))
	svc := project.NewProjectService(repo, log)

	return svc.CreateProject(name)
}

func projectDelete(args []string) error {
	if len(args) < 1 {
		return ErrNoProjectName
	}

	lookup := args[0]

	home, err := common.HomeDir()
	if err != nil {
		return err
	}

	repo := persistence.NewFileProjectRepository(filepath.Join(home, ".drudge", "projects"))
	svc := project.NewProjectService(repo, nil)

	proj, err := svc.LookupProject(lookup)
	if err != nil {
		return fmt.Errorf("could not find project %q: %w", lookup, err)
	}

	name := proj.Name

	force := false
	for _, a := range args {
		if a == "--force" || a == "-f" {
			force = true
		}
	}

	if !force {
		fmt.Printf("This will permanently delete project %q\nAre you sure? [y/N]: ", name)
		var response string
		if _, err := fmt.Scanln(&response); err != nil && err.Error() != "unexpected newline" {
			return fmt.Errorf("could not read confirmation: %w", err)
		}
		if response != "y" && response != "Y" {
			fmt.Println("Aborted")
			return nil
		}
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

	home, err := common.HomeDir()
	if err != nil {
		return err
	}

	repo := persistence.NewFileProjectRepository(filepath.Join(home, ".drudge", "projects"))
	log := common.NewLogger("")
	svc := project.NewProjectService(repo, log)

	return svc.RenameProject(oldName, newName)
}

func projectList() error {
	log := common.NewLogger("")

	home, err := common.HomeDir()
	if err != nil {
		return err
	}

	repo := persistence.NewFileProjectRepository(filepath.Join(home, ".drudge", "projects"))
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
