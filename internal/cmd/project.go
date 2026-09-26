package cmd

import (
	"flag"
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

// unresolvedBranch stands in for a default branch drudge could not work out.
const unresolvedBranch = "unresolved"

var ProjectCmd = &Cmd{
	Name: "project",
	Desc: "Project management commands",
	Subcommands: []*Cmd{
		{
			Name: "create",
			Args: []string{projectNameArg},
			Desc: "Record a project without linking a directory to it",
			Help: "Record a project in the drudge home directory without linking any directory to it.\n" +
				"Run drg project init inside the project directory to record a project and link the directory in one go.",
			Setup: func(*flag.FlagSet) func(args []string) error { return projectCreate },
		},
		{
			Name: "init",
			Args: []string{projectNameArg},
			Desc: "Record a project and link the current directory to it",
			Help: "Record a project and link the current directory to it. Task commands run here work on that project.\n" +
				"A directory that is a git repository becomes the one repository of the project.\n" +
				"Otherwise every subdirectory that is a git repository becomes one, and a directory holding none is refused.\n" +
				"It prints each repository with the default branch that task branches are cut from.",
			Setup: func(*flag.FlagSet) func(args []string) error { return projectInit },
		},
		{
			Name: "delete",
			Args: []string{projectNameArg},
			Desc: "Delete a project and its tasks",
			Help: "Delete a project and everything drudge keeps for it in the drudge home directory, its tasks included. It asks first.\n" +
				"The project may be named by its name or its slug.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				isForced := fs.Bool(forceFlagName, false, "Delete the project without asking")
				alias(fs, forceFlagShortName, forceFlagName)
				return func(args []string) error { return projectDelete(args[0], *isForced) }
			},
		},
		{
			Name: "rename",
			Args: []string{"old-name", "new-name"},
			Desc: "Give a project a new name",
			Help: "Give a project a new name. Its slug and its files stay, so directories linked to it keep working.\n" +
				"The project may be named by its name or its slug. A name another project goes by is refused.",
			Setup: func(*flag.FlagSet) func(args []string) error { return projectRename },
		},
		{
			Name:  "list",
			Desc:  "List the projects",
			Help:  "List the projects drudge knows about, with the slug and the name of each.",
			Setup: func(*flag.FlagSet) func(args []string) error { return projectList },
		},
	},
}

const projectNameArg = "name"

func projectCreate(args []string) error {
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

	_, repositories, err := svc.InitProject(name, projectDir)
	if err != nil {
		return err
	}

	log.Info("Initialized project %s in %s", name, common.DotDrudgeDirName)
	printRepositories(log, svc.ResolveRepositories(projectDir, repositories))
	return nil
}

// printRepositories lists the repositories of a project with the branch each
// one cuts work from. A repository whose default branch does not resolve is
// listed as unresolved and the fix goes to stderr, leaving the recorded list
// for the user to edit.
func printRepositories(log *common.Logger, resolved []project.ResolvedRepository) {
	columns := []column{
		{Title: "REPOSITORY", Width: 30},
		{Title: "DEFAULT BRANCH"},
	}

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

func projectDelete(lookup string, isForced bool) error {
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

	if !isForced {
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
	oldName, newName := args[0], args[1]

	log := common.NewLogger("")
	svc, err := newProjectService(log)
	if err != nil {
		return err
	}

	return svc.RenameProject(oldName, newName)
}

func projectList(args []string) error {
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
		log.Info("No projects yet, run drg project init <name> in a project directory to create one")
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
