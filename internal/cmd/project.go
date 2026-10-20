package cmd

import (
	"flag"
	"fmt"
	"slices"

	"github.com/IgorBolotnikov/DRUDGE/internal/cmd/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
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
				"It writes the setup commands, copy paths and environment variable names it detects for each repository, like drg project detect does.\n" +
				"It prints each repository with the default branch that task branches are cut from.",
			Setup: func(*flag.FlagSet) func(args []string) error { return projectInit },
		},
		{
			Name: "detect",
			Desc: "Detect the setup commands and copy paths of the repositories",
			Help: "Detect the setup commands and copy paths of every repository of the project from the files in its root, and write them to the setup and copy keys of each repository in the local config.\n" +
				"A version file like .tool-versions, mise.toml, .nvmrc or .python-version gets mise and the pinned toolchains installed.\n" +
				"The first lockfile found gets its packages installed. A repository that matches nothing gets an empty list.\n" +
				"A .pre-commit-config.yaml gets pre-commit and its hooks installed into the worktree only, through a hooks directory of the sandbox. Hooks a package manager installs, like husky, get no lines.\n" +
				"Files git ignores whose name starts with .env, like .env and .env.local, and a .envs directory get copied. A .envrc is never copied.\n" +
				"Every ${NAME} in a .npmrc is added to " + drudger.EnvKey + " of the local config, for every repository. Names already there stay and no name is ever removed, --force included.\n" +
				"It reads only the root of each repository.\n" +
				"A repository that has a setup key keeps it and its copy key, an empty list included, unless --force is given.\n" +
				"A repository with no setup key keeps a copy key it has, unless --force is given.",
			Setup: func(fs *flag.FlagSet) func(args []string) error {
				var flags projectDetectFlags
				flags.declare(fs)
				return func([]string) error { return projectDetect(flags.options()) }
			},
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

	out.Header("Initializing project %s in %s", name, common.DotDrudgeDirName)
	_, repositories, detections, err := svc.InitProject(name, projectDir)
	if err != nil {
		return err
	}

	resolved := svc.ResolveRepositories(projectDir, repositories)
	for _, repository := range resolved {
		if repository.Problem != nil {
			out.Warn("%s", repository.Problem)
		}
	}
	out.Result("Initialized project %s", name)
	printSetupDetections(out, detections, false)
	out.View(repositoryLines(resolved))
	return nil
}

// projectDetectFlags are the flags of drg project detect.
type projectDetectFlags struct {
	isForced bool
	isDryRun bool
}

func (flags *projectDetectFlags) declare(fs *flag.FlagSet) {
	fs.BoolVar(&flags.isForced, forceFlagName, false, "Replace the setup and copy paths of every repository, existing ones included")
	alias(fs, forceFlagShortName, forceFlagName)
	fs.BoolVar(&flags.isDryRun, dryRunFlagName, false, "Print the detected setup and copy paths and write nothing")
}

func (flags *projectDetectFlags) options() project.DetectSetupOptions {
	return project.DetectSetupOptions{IsForced: flags.isForced, IsDryRun: flags.isDryRun}
}

func projectDetect(options project.DetectSetupOptions) error {
	projectDir, err := common.WorkDir()
	if err != nil {
		return err
	}

	out := newCommandPrinter()
	svc, err := newProjectService(out)
	if err != nil {
		return err
	}

	detections, err := svc.DetectSetup(projectDir, options)
	if err != nil {
		return err
	}

	printSetupDetections(out, detections, options.IsDryRun)
	switch {
	case options.IsDryRun:
		out.SkipResult("Dry run, wrote nothing")
	case !slices.ContainsFunc(detections, func(detection project.SetupDetection) bool { return !detection.IsKept || len(detection.EnvNames) > 0 }):
		out.SkipResult("Every repository has a setup already, --%s replaces it", forceFlagName)
	default:
		out.Result("Wrote the detected setup to %s", common.LocalConfigPath())
	}
	return nil
}

// printSetupDetections prints one group per repository, with every setup line
// detected for it followed by the file that triggered it, and then every copy
// path and every environment variable name followed by the file that names it.
// A dry run prints the lines as skipped.
func printSetupDetections(out *printer.Printer, detections []project.SetupDetection, isDryRun bool) {
	for _, detection := range detections {
		out.Header("Setup of %s", detection.Repository)
		switch {
		case detection.IsKept:
			out.Skip("Kept the existing setup")
		case len(detection.Lines) == 0 && isDryRun:
			out.Skip("Nothing to set up")
		case len(detection.Lines) == 0:
			out.Done("Nothing to set up, wrote an empty list")
		}
		for _, line := range detection.Lines {
			trigger := out.Theme().Paint(theme.Stdout, theme.RoleMuted, line.Trigger)
			if isDryRun {
				out.Skip("%s  %s", line.Command, trigger)
				continue
			}
			out.Done("%s  %s", line.Command, trigger)
		}
		if detection.IsCopyKept {
			out.Skip("Kept the existing copy paths")
		}
		for _, path := range detection.CopyPaths {
			if isDryRun {
				out.Skip("Copy %s", path)
				continue
			}
			out.Done("Copy %s", path)
		}
		for _, envName := range detection.EnvNames {
			trigger := out.Theme().Paint(theme.Stdout, theme.RoleMuted, envName.Trigger)
			if isDryRun {
				out.Skip("Add %s to %s  %s", envName.Name, drudger.EnvKey, trigger)
				continue
			}
			out.Done("Add %s to %s  %s", envName.Name, drudger.EnvKey, trigger)
		}
	}
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
			out.Skip("Left project %s alone", name)
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
		out.Skip("No projects yet, run drg project init <name> in a project directory to create one")
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
	out.View(append(lines, pageFooterLines(out.Theme(), listed.Number, listed.TotalPages)...))
	return nil
}
