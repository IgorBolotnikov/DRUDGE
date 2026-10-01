package cmd

import (
	"flag"
	"fmt"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

// unresolvedBranch stands in for a default branch drudge could not work out.
const unresolvedBranch = "unresolved"

var ProjectCmd = &Cmd{
	Name: "project",
	Desc: "Project management commands",
	Subcommands: []*Cmd{
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
			Name: "list",
			Desc: "List the projects",
			Help: "List the projects drudge knows about, with the slug and the name of each, one page at a time.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				var flags pageFlags
				flags.declare(fs, config.ProjectPageSizeKey+" from the global config")
				return func([]string) error { return projectList(flags) }
			},
		},
	},
}

const projectNameArg = "name"

func projectInit(args []string) error {
	name := args[0]

	projectDir, err := common.WorkDir()
	if err != nil {
		return err
	}

	out := newCommandPrinter()
	svc, err := newProjectService(out)
	if err != nil {
		return err
	}

	out.header("Initializing project %s in %s", name, common.DotDrudgeDirName)
	_, repositories, err := svc.InitProject(name, projectDir)
	if err != nil {
		return err
	}

	resolved := svc.ResolveRepositories(projectDir, repositories)
	for _, repository := range resolved {
		if repository.Problem != nil {
			out.warn("%s", repository.Problem)
		}
	}
	out.result("Initialized project %s", name)
	out.view(repositoryLines(resolved))
	return nil
}

// repositoryLines lists the repositories of a project with the branch each
// one cuts work from. A repository whose default branch does not resolve is
// listed as unresolved.
func repositoryLines(resolved []project.ResolvedRepository) []string {
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
	return listLines("Repositories", len(rows), columns, rows, nil)
}

func projectDelete(lookup string, isForced bool) error {
	out := newCommandPrinter()
	svc, err := newProjectService(out)
	if err != nil {
		return err
	}

	proj, err := svc.LookupProject(lookup)
	if err != nil {
		return fmt.Errorf("could not find project %q: %w", lookup, err)
	}

	name := proj.Name

	if !isForced {
		isConfirmed, err := ConfirmDeletion(out, fmt.Sprintf("project %s", name))
		if err != nil {
			return err
		}
		if !isConfirmed {
			out.skip("Left project %s alone", name)
			return nil
		}
	}

	if err := svc.DeleteProject(proj); err != nil {
		return fmt.Errorf("could not delete project %q: %w", name, err)
	}
	return nil
}

func projectRename(args []string) error {
	oldName, newName := args[0], args[1]

	svc, err := newProjectService(newCommandPrinter())
	if err != nil {
		return err
	}

	return svc.RenameProject(oldName, newName)
}

func projectList(flags pageFlags) error {
	globalCfg, err := config.Load()
	if err != nil {
		return err
	}
	size, err := flags.pageSize(config.ResolveProjectPageSize(globalCfg))
	if err != nil {
		return err
	}

	out := newCommandPrinter()
	svc, err := newProjectService(out)
	if err != nil {
		return err
	}

	listed, err := svc.ListProjects(flags.number, size)
	if err != nil {
		return err
	}

	if listed.TotalItems == 0 {
		out.skip("No projects yet, run drg project init <name> in a project directory to create one")
		return nil
	}

	columns := []column{
		{Title: "SLUG", Width: 20},
		{Title: "NAME"},
	}
	rows := make([][]string, 0, len(listed.Items))
	for _, p := range listed.Items {
		rows = append(rows, []string{p.Slug, p.Name})
	}

	lines := listLines("Projects", listed.TotalItems, columns, rows, nil)
	out.view(append(lines, pageFooterLines(out.theme, listed.Number, listed.TotalPages)...))
	return nil
}
