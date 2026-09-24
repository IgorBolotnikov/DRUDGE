package cmd

import (
	"errors"
	"fmt"
	"strings"

	"drudge/internal/adapters/persistence"
	"drudge/internal/common"
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

const (
	projectCreateSubcommand = "create"
	projectInitSubcommand   = "init"
	projectDeleteSubcommand = "delete"
	projectRenameSubcommand = "rename"
	projectListSubcommand   = "list"
)

const (
	projectUsage       = "usage: drg project <create|init|delete|rename|list>"
	projectCreateUsage = "usage: drg project create <name>"
	projectInitUsage   = "usage: drg project init <name>"
	projectDeleteUsage = "usage: drg project delete <name> [" + forceFlag + "]"
	projectRenameUsage = "usage: drg project rename <old-name> <new-name>"
	projectListUsage   = "usage: drg project list"
)

const projectNameArg = "project name"

func runProject(args []string) error {
	if len(args) < 1 {
		return errors.New(projectUsage)
	}

	switch args[0] {
	case helpFlag, helpFlagShort:
		printProjectHelp()
		return nil
	case projectCreateSubcommand:
		return projectCreate(args[1:])
	case projectInitSubcommand:
		return projectInit(args[1:])
	case projectDeleteSubcommand:
		return projectDelete(args[1:])
	case projectRenameSubcommand:
		return projectRename(args[1:])
	case projectListSubcommand:
		return projectList(args[1:])
	default:
		return fmt.Errorf("unknown project subcommand %q, %s", args[0], projectUsage)
	}
}

func printProjectHelp() {
	fmt.Println(projectUsage)
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Printf("  %-8s Record a project without linking a directory to it\n", projectCreateSubcommand)
	fmt.Printf("  %-8s Record a project and link the current directory to it\n", projectInitSubcommand)
	fmt.Printf("  %-8s Delete a project and its tasks\n", projectDeleteSubcommand)
	fmt.Printf("  %-8s Give a project a new name\n", projectRenameSubcommand)
	fmt.Printf("  %-8s List the projects\n", projectListSubcommand)
	fmt.Println()
	fmt.Printf("Run drg project <subcommand> %s for the details of one.\n", helpFlag)
}

// parseProjectArgs reads the arguments of a project subcommand, one per entry
// of argNames, and whether the force flag was given. It refuses a missing or
// extra argument, and any flag other than the force flag of a subcommand that
// takes it.
func parseProjectArgs(args []string, argNames []string, isForceAllowed bool, usage string) ([]string, bool, error) {
	var values []string
	isForced := false

	for _, arg := range args {
		switch {
		case isForceAllowed && (arg == forceFlag || arg == forceFlagShort):
			isForced = true
		case strings.HasPrefix(arg, "-"):
			return nil, false, fmt.Errorf("unknown flag %q, %s", arg, usage)
		case len(values) == len(argNames):
			return nil, false, fmt.Errorf("unexpected argument %q, %s", arg, usage)
		default:
			values = append(values, arg)
		}
	}

	if len(values) < len(argNames) {
		return nil, false, fmt.Errorf("%s is required, %s", argNames[len(values)], usage)
	}
	return values, isForced, nil
}

func projectCreate(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(projectCreateUsage)
		fmt.Println()
		fmt.Println("Record a project in the drudge home directory without linking any directory to it.")
		fmt.Println("Run drg project init inside the project directory to record a project and link the directory in one go.")
		return nil
	}

	values, _, err := parseProjectArgs(args, []string{projectNameArg}, false, projectCreateUsage)
	if err != nil {
		return err
	}
	name := values[0]

	log := common.NewLogger("")
	svc, err := newProjectService(log)
	if err != nil {
		return err
	}

	_, err = svc.CreateProject(name)
	return err
}

func projectInit(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(projectInitUsage)
		fmt.Println()
		fmt.Println("Record a project and link the current directory to it. Task commands run here work on that project.")
		fmt.Println("A directory that is a git repository becomes the one repository of the project.")
		fmt.Println("Otherwise every subdirectory that is a git repository becomes one, and a directory holding none is refused.")
		fmt.Println("It prints each repository with the default branch that task branches are cut from.")
		return nil
	}

	values, _, err := parseProjectArgs(args, []string{projectNameArg}, false, projectInitUsage)
	if err != nil {
		return err
	}
	name := values[0]

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

func projectDelete(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(projectDeleteUsage)
		fmt.Println()
		fmt.Println("Delete a project and everything drudge keeps for it in the drudge home directory, its tasks included. It asks first.")
		fmt.Println("The project may be named by its name or its slug.")
		fmt.Println()
		fmt.Println("Options:")
		fmt.Printf("  %s  Delete the project without asking\n", forceFlag)
		return nil
	}

	values, isForced, err := parseProjectArgs(args, []string{projectNameArg}, true, projectDeleteUsage)
	if err != nil {
		return err
	}
	lookup := values[0]

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
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(projectRenameUsage)
		fmt.Println()
		fmt.Println("Give a project a new name. Its slug follows the new name, and a slug another project holds is refused.")
		return nil
	}

	values, _, err := parseProjectArgs(args, []string{"old name", "new name"}, false, projectRenameUsage)
	if err != nil {
		return err
	}
	oldName, newName := values[0], values[1]

	log := common.NewLogger("")
	svc, err := newProjectService(log)
	if err != nil {
		return err
	}

	return svc.RenameProject(oldName, newName)
}

func projectList(args []string) error {
	if hasFlag(args, helpFlag) || hasFlag(args, helpFlagShort) {
		fmt.Println(projectListUsage)
		fmt.Println()
		fmt.Println("List the projects drudge knows about, with the slug and the name of each.")
		return nil
	}

	if _, _, err := parseProjectArgs(args, nil, false, projectListUsage); err != nil {
		return err
	}

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
