package cmd

import (
	"fmt"

	"drudge/internal/adapters/persistence"
	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/project"
)

// unresolvedBranch stands in for a default branch drudge could not work out.
const unresolvedBranch = "unresolved"

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
	svc, err := newProjectService(log)
	if err != nil {
		return err
	}

	_, err = svc.CreateProject(name)
	return err
}

func projectInit(args []string) error {
	if len(args) < 1 {
		return ErrNoProjectName
	}

	name := args[0]

	projectDir, err := common.WorkDir()
	if err != nil {
		return err
	}

	log := common.NewLogger("")
	svc, err := newProjectService(log)
	if err != nil {
		return err
	}

	repositories, err := svc.DiscoverRepositories(projectDir)
	if err != nil {
		return err
	}

	proj, err := svc.CreateProject(name)
	if err != nil {
		return err
	}

	cfg := config.LocalConfig{ProjectSlug: proj.Slug, Repositories: repositories}
	if err := cfg.Save(); err != nil {
		return err
	}

	log.Info("Initialized project %s in %s", name, common.DotDrudgeDirName)
	printRepositories(log, svc, projectDir, repositories)
	return nil
}

// printRepositories lists the repositories of a project with the branch each
// one cuts work from. A repository whose default branch does not resolve is
// listed as unresolved and the fix goes to stderr, leaving the recorded list
// for the user to edit.
func printRepositories(log *common.Logger, svc *project.ProjectService, projectDir string, repositories []config.Repository) {
	columns := []column{
		{Title: "REPOSITORY", Width: 30},
		{Title: "DEFAULT BRANCH"},
	}

	resolved := svc.ResolveRepositories(projectDir, repositories)

	rows := make([][]string, 0, len(resolved))
	for _, repository := range resolved {
		branch := repository.DefaultBranch
		if repository.Problem != nil {
			branch = unresolvedBranch
		}
		rows = append(rows, []string{repository.Repository.Path, branch})
	}
	printList(log, "Repositories", columns, rows)

	for _, repository := range resolved {
		if repository.Problem != nil {
			log.Error("%s", repository.Problem)
		}
	}
}

func projectDelete(args []string) error {
	if len(args) < 1 {
		return ErrNoProjectName
	}

	lookup := args[0]

	log := common.NewLogger("")
	repo := persistence.NewFileProjectRepository("")
	svc, err := newProjectService(log)
	if err != nil {
		return err
	}

	proj, err := svc.LookupProject(lookup)
	if err != nil {
		return fmt.Errorf("could not find project %q: %w", lookup, err)
	}

	name := proj.Name

	if !HasForceFlag(args) {
		isConfirmed, err := ConfirmDeletion(fmt.Sprintf("project %q", name))
		if err != nil {
			return err
		}
		if !isConfirmed {
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

	log := common.NewLogger("")
	svc, err := newProjectService(log)
	if err != nil {
		return err
	}

	return svc.RenameProject(oldName, newName)
}

func projectList() error {
	log := common.NewLogger("")
	svc, err := newProjectService(log)
	if err != nil {
		return err
	}

	projects, err := svc.ListProjects()
	if err != nil {
		return fmt.Errorf("could not list projects: %w", err)
	}

	if len(projects) == 0 {
		log.Info("No projects found")
		return nil
	}

	columns := []column{
		{Title: "SLUG", Width: 20},
		{Title: "NAME"},
	}
	rows := make([][]string, 0, len(projects))
	for _, p := range projects {
		rows = append(rows, []string{p.Slug, p.Name})
	}

	printList(log, "Projects", columns, rows)
	return nil
}
