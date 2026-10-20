package drudger

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// CommandRunner runs the commands that put a Drudger to work.
type CommandRunner interface {
	// Run waits for a command to finish and hands back what it wrote to
	// stdout and to stderr. Writing to stderr is not a failure on its own:
	// that may be where the sandbox prints its results. A command that runs
	// longer than timeout is killed and the error wraps
	// context.DeadlineExceeded.
	Run(argv []string, timeout time.Duration) (stdout string, stderr string, err error)
	// RunEchoed is Run that also hands echo each line the command writes to
	// stdout or to stderr while it runs.
	RunEchoed(argv []string, timeout time.Duration, echo func(line string)) (stdout string, stderr string, err error)
	// Start runs the command and returns immediately.
	Start(argv []string) error
}

// timedOut tells whether a command was killed for outrunning its timeout.
func timedOut(err error) bool {
	return errors.Is(err, context.DeadlineExceeded)
}

// Pieces of an sbx invocation.
const (
	sbxBinary           = "sbx"
	sbxLsSubcommand     = "ls"
	sbxCreateSubcommand = "create"
	sbxExecSubcommand   = "exec"
	sbxRmSubcommand     = "rm"
	sbxJSONFlag         = "--json"
	sbxNameFlag         = "--name"
	sbxDetachedFlag     = "-d"
	sbxWorkdirFlag      = "-w"
	// sbx asks for confirmation before removing a sandbox and this flag is used
	// only to skip the confirmation prompt. The flag is always passed since
	// drudge has no terminal to confirm the action. Drudge has its own
	// safety check for that.
	sbxForceFlag = "--force"

	sbxHarnessClaude = "claude"
	// This harness is to be implemented later
	sbxHarnessOpencode = "opencode"

	// sbxStatusRunning is the sandbox status that means a process can be
	// running inside it. Every other status means nothing is.
	sbxStatusRunning = "running"
)

// Pieces of the shell invocation that launches an agent inside a sandbox.
const (
	shellBinary      = "sh"
	shellCommandFlag = "-c"

	claudeBinary            = "claude"
	claudePromptFlag        = "-p"
	claudeOutputFormatFlag  = "--output-format"
	claudeStreamJSONFormat  = "stream-json"
	claudeVerboseFlag       = "--verbose"
	claudePermissionFlag    = "--permission-mode"
	claudeBypassPermissions = "bypassPermissions"
	claudeModelFlag         = "--model"
)

// What setup leaves for later setup commands and for the agent. Both paths
// are relative to the sandbox home, outside every worktree.
const (
	drudgePathEnv  = "DRUDGE_PATH"
	drudgeHooksEnv = "DRUDGE_HOOKS"
	// sandboxPathFile lists one directory per line to prepend to PATH.
	sandboxPathFile = ".drudge/path"
	// sandboxHooksDir holds one git hooks directory per repository.
	sandboxHooksDir = ".drudge/hooks"
)

// prependListedPathScript prepends every directory the path file lists to
// PATH, the first line first. A missing file and empty lines change nothing.
var prependListedPathScript = fmt.Sprintf(`if [ -f "$%[1]s" ]; then
  drudge_prefix=
  while IFS= read -r drudge_dir || [ -n "$drudge_dir" ]; do
    if [ -n "$drudge_dir" ]; then drudge_prefix="$drudge_prefix$drudge_dir:"; fi
  done < "$%[1]s"
  export PATH="$drudge_prefix$PATH"
fi
`, drudgePathEnv)

// Drudger name prefixes.
const (
	claudeCodeDrudgerPrefix = "drudge-claude"
	opencodeDrudgerPrefix   = "drudge-opencode"
	unknownDrudgerPrefix    = "drudge-unknown"
)

// unknownProjectSlug stands in for a project slug that normalizes to nothing.
const unknownProjectSlug = "unknown"

// What sbx writes to stderr about its own daemon.
const (
	// The first sbx call after a boot brings the daemon up and says so. The
	// call itself still succeeds.
	sbxDaemonStartingNotice = "Starting sandboxd daemon..."
	// A daemon that will not come up fails the call with this in the message.
	sbxDaemonFailureNotice = "ensure daemon"
	// What a user runs to look at the daemon themselves.
	sbxDaemonStatusCommand = "sbx daemon status"
)

// All the sandbox code here is relared to a sigle environment: `docker sbx`.
// TODO: move it to its own package

// sandboxCommand is a sandbox command and how long it may run before DRUDGE
// kills it.
type sandboxCommand struct {
	argv    []string
	timeout time.Duration
}

// sandboxPlan is the ordered set of commands that puts a Drudger to work.
type sandboxPlan struct {
	inspect sandboxCommand // lists sandboxes so drudge can tell whether this Drudger's sandbox exists
	create  sandboxCommand // creates the sandbox, runs only when it does not exist yet
	reset   sandboxCommand // empties the path file and creates the hooks directories, runs before setup
	setup   []setupCommand // prepares the repositories the task changes, in config order
	start   []string       // starts the agent and passes the prompt, always runs and is never waited for
}

// setupCommand is one setup command of a repository, as the config lists it
// and as it runs in the sandbox.
type setupCommand struct {
	sandboxCommand
	repository string
	command    string
}

// sandboxListing is what `sbx ls --json` reports.
type sandboxListing struct {
	Sandboxes []sandbox `json:"sandboxes"`
}

// sandbox is one entry of a sandbox listing.
type sandbox struct {
	Name       string   `json:"name"`
	Status     string   `json:"status"`
	Workspaces []string `json:"workspaces"`
}

// pickInspectCommand builds the command that lists the sandboxes of the
// configured environment.
func (service *DrudgerService) pickInspectCommand() (sandboxCommand, error) {
	env := service.settings.Env

	if env == EnvDockerSbx {
		return service.sbxInspectCommand(), nil
	}

	return sandboxCommand{}, fmt.Errorf("DRUDGE does not know how to list the sandboxes of environment %q, check the Drudger settings in the config", env)
}

// sbxInspectCommand lists the sandboxes of an sbx environment.
func (service *DrudgerService) sbxInspectCommand() sandboxCommand {
	return sandboxCommand{
		argv:    []string{sbxBinary, sbxLsSubcommand, sbxJSONFlag},
		timeout: service.settings.SandboxTimeouts.List,
	}
}

// pickDrudgerCommand builds the commands that ensure the sandbox exists, run
// the setup commands of the repositories a task changes and put an agent to
// work on the configured prompt. The sandbox is created over every mount, and
// the agent works in the workspace root.
func (service *DrudgerService) pickDrudgerCommand(sandboxName string, workspaceRoot string, mounts []string, runDir string, changed []repositoryWorktree) (sandboxPlan, error) {
	env := service.settings.Env
	harness := service.settings.Harness

	// TODO: at some point need to move concrete implementations to adapters
	if env == EnvDockerSbx && harness == HarnessClaudeCode {
		create := []string{sbxBinary, sbxCreateSubcommand, sbxHarnessClaude}
		create = append(create, mounts...)
		create = append(create, sbxNameFlag, sandboxName)

		return sandboxPlan{
			inspect: service.sbxInspectCommand(),
			create: sandboxCommand{
				argv:    create,
				timeout: service.settings.SandboxTimeouts.Create,
			},
			reset: sandboxCommand{
				argv:    sbxExecCommand(sbxExec{sandbox: sandboxName, script: formatSetupReset(changed)}),
				timeout: service.settings.SandboxTimeouts.Setup,
			},
			setup: service.sbxSetupCommands(sandboxName, changed),
			start: sbxExecCommand(sbxExec{
				sandbox:    sandboxName,
				script:     formatLauncher(workspaceRoot, runDir, service.settings.Model),
				isDetached: true,
			}),
		}, nil
	}

	return sandboxPlan{}, fmt.Errorf("DRUDGE does not know how to start harness %q in environment %q, check the Drudger settings in the config", harness, env)
}

// sbxSetupCommands builds the setup commands of every given repository. Each
// one runs in the worktree of its repository, which the sandbox mounts at its
// host path, with the path file and the hooks directory of its repository
// exported and the listed directories on PATH.
func (service *DrudgerService) sbxSetupCommands(sandboxName string, repositories []repositoryWorktree) []setupCommand {
	var commands []setupCommand
	for _, repository := range repositories {
		environment := formatPathPreamble() + "export " + drudgeHooksEnv + "=" + sandboxHomePath(hooksDirOf(repository.Path)) + "\n"
		for _, command := range repository.Setup {
			commands = append(commands, setupCommand{
				sandboxCommand: sandboxCommand{
					argv:    sbxExecCommand(sbxExec{sandbox: sandboxName, workdir: repository.Worktree, script: environment + command}),
					timeout: service.settings.SandboxTimeouts.Setup,
				},
				repository: repository.Name,
				command:    command,
			})
		}
	}
	return commands
}

// formatSetupReset renders the script that empties the path file and creates
// the hooks directory of every repository with setup commands. A launch runs
// it even with no setup, so the agent never reads directories a previous
// launch listed.
func formatSetupReset(repositories []repositoryWorktree) string {
	dirs := []string{sandboxHomePath(path.Dir(sandboxPathFile))}
	for _, repository := range repositories {
		if len(repository.Setup) > 0 {
			dirs = append(dirs, sandboxHomePath(hooksDirOf(repository.Path)))
		}
	}
	return "mkdir -p " + strings.Join(dirs, " ") + " && : > " + sandboxHomePath(sandboxPathFile)
}

// formatPathPreamble renders the shell lines that export the path file and put
// the directories it lists on PATH.
func formatPathPreamble() string {
	return "export " + drudgePathEnv + "=" + sandboxHomePath(sandboxPathFile) + "\n" + prependListedPathScript
}

// hooksDirOf names the hooks directory of a repository under the sandbox home.
// The repository path is escaped into one file name. A leading dot is escaped
// too, so the project directory itself, recorded as ".", gets a directory of
// its own.
func hooksDirOf(repositoryPath string) string {
	name := url.PathEscape(repositoryPath)
	if strings.HasPrefix(name, ".") {
		name = "%2E" + name[1:]
	}
	return path.Join(sandboxHooksDir, name)
}

// sandboxHomePath renders a path under the sandbox home for a shell in the
// sandbox, which expands the home and reads the rest literally.
func sandboxHomePath(relative string) string {
	return `"$HOME"/` + shellQuote(relative)
}

// sbxExec is a shell script to run in a sandbox. An empty workdir leaves the
// working directory to sbx. A detached script is not waited for.
type sbxExec struct {
	sandbox    string
	workdir    string
	script     string
	isDetached bool
}

// sbxExecCommand builds the sbx exec command line that runs a script in a
// sandbox with sh -c. Every command drudge runs inside a sandbox is built here.
func sbxExecCommand(invocation sbxExec) []string {
	argv := []string{sbxBinary, sbxExecSubcommand}
	if invocation.isDetached {
		argv = append(argv, sbxDetachedFlag)
	}
	if invocation.workdir != "" {
		argv = append(argv, sbxWorkdirFlag, invocation.workdir)
	}
	return append(argv, invocation.sandbox, shellBinary, shellCommandFlag, invocation.script)
}

// pickRemoveCommand builds the command that deletes a Drudger's sandbox.
func (service *DrudgerService) pickRemoveCommand(sandboxName string) (sandboxCommand, error) {
	env := service.settings.Env

	if env == EnvDockerSbx {
		return sandboxCommand{
			argv:    []string{sbxBinary, sbxRmSubcommand, sbxForceFlag, sandboxName},
			timeout: service.settings.SandboxTimeouts.Remove,
		}, nil
	}

	return sandboxCommand{}, fmt.Errorf("DRUDGE does not know how to remove a sandbox in environment %q, check the Drudger settings in the config", env)
}

// formatLauncher renders the shell script that runs the agent in a sandbox.
// The agent works in the workspace and its output lands in the run directory,
// so every path in it is absolute. The prompt is read from a file to keep an
// arbitrarily long, arbitrarily quoted prompt out of the command line.
//
// Permission prompts are explicitly bypassed so that an agent does not stall
// because it waits for a tool call approval. The sandbox is the security
// boundary here.
//
// An empty model leaves the choice of model to the harness.
//
// The agent gets the directories the path file lists on PATH.
//
// The exit file is written last and is the only marker of a finished run.
func formatLauncher(workspace, runDir, model string) string {
	agentArgs := []string{
		claudeBinary,
		claudePromptFlag, catPrompt(runDir),
		claudeOutputFormatFlag, claudeStreamJSONFormat,
		claudeVerboseFlag,
		claudePermissionFlag, claudeBypassPermissions,
	}
	if model != "" {
		agentArgs = append(agentArgs, claudeModelFlag, shellQuote(model))
	}
	agentCommand := strings.Join(agentArgs, " ")

	return fmt.Sprintf(
		"%scd %s || exit 1\n%s > %s 2> %s\necho $? > %s\n",
		formatPathPreamble(),
		shellQuote(workspace),
		agentCommand,
		shellQuote(common.RunStreamPath(runDir)),
		shellQuote(common.RunStderrPath(runDir)),
		shellQuote(common.RunExitPath(runDir)),
	)
}

// catPrompts returns a cat command reading a quoted and escaped prompt.
func catPrompt(runDir string) string {
	return `"$(cat ` + shellQuote(common.RunPromptPath(runDir)) + `)"`
}

// shellQuote wraps a value in single quotes so a shell reads it literally.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// readRunningSandboxes maps each sandbox of a listing to whether it is
// running. A sandbox the listing does not hold is absent from the map, which
// reads as not running.
func readRunningSandboxes(listing string) (map[string]bool, error) {
	var parsed sandboxListing
	if err := json.Unmarshal([]byte(listing), &parsed); err != nil {
		return nil, fmt.Errorf("could not parse the sandbox listing: %w", err)
	}

	running := make(map[string]bool, len(parsed.Sandboxes))
	for _, candidate := range parsed.Sandboxes {
		running[candidate.Name] = candidate.Status == sbxStatusRunning
	}
	return running, nil
}

// findSandbox picks the named sandbox out of a sandbox listing. A nil result
// means the listing does not hold it.
func findSandbox(listing string, name string) (*sandbox, error) {
	var parsed sandboxListing
	if err := json.Unmarshal([]byte(listing), &parsed); err != nil {
		return nil, fmt.Errorf("could not parse the sandbox listing: %w", err)
	}

	for _, candidate := range parsed.Sandboxes {
		if candidate.Name == name {
			return &candidate, nil
		}
	}
	return nil, nil
}

// checkSandboxWorkspace guards against an agent working in the wrong checkout.
// A sandbox is reused only when it holds every mount the run needs, and any
// mount beyond them is left alone.
//
// The runs mount is checked with the rest. A sandbox without it takes the
// agent's stream into a path that exists only inside the container, so no run
// directory appears on the host and the launch dies at the grace period.
func checkSandboxWorkspace(existing *sandbox, mounts []string, slot int) error {
	held := make(map[string]bool, len(existing.Workspaces))
	for _, mount := range existing.Workspaces {
		held[filepath.Clean(mount)] = true
	}

	missing := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		if !held[filepath.Clean(mount)] {
			missing = append(missing, mount)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf(
		"sandbox %s is mounted on %s and does not hold %s, run %s %d to delete it so DRUDGE can recreate it over this workspace",
		existing.Name, formatMounts(existing.Workspaces), strings.Join(missing, ", "), nukeCommand, slot,
	)
}

// formatMounts renders a sandbox's workspace mounts for an error message.
func formatMounts(mounts []string) string {
	if len(mounts) == 0 {
		return "nothing"
	}
	return strings.Join(mounts, ", ")
}

// formatDrudgerName names the sandbox a Drudger slot works in.
func formatDrudgerName(projectSlug string, drudgerSlot int, harness Harness) string {
	prefix := unknownDrudgerPrefix
	switch harness {
	case HarnessClaudeCode:
		prefix = claudeCodeDrudgerPrefix
	case HarnessOpencode:
		prefix = opencodeDrudgerPrefix
	}
	return fmt.Sprintf("%s-%s-%d", prefix, normaliseNameSlug(projectSlug), drudgerSlot)
}

// normaliseNameSlug normalizes the Drudger name to include only:
// lowercase letters, numbers, hyphens and periods. Anything else
// becomes a hyphen, and multiple consecutive hyphens collapse into one.
func normaliseNameSlug(projectSlug string) string {
	var name strings.Builder
	isAfterSeparator := false

	for _, char := range strings.ToLower(projectSlug) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '.' {
			name.WriteRune(char)
			isAfterSeparator = false
			continue
		}
		if !isAfterSeparator {
			name.WriteByte('-')
			isAfterSeparator = true
		}
	}

	slug := strings.Trim(name.String(), "-.")
	if slug == "" {
		return unknownProjectSlug
	}
	return slug
}

// daemonJustStarted tells whether an sbx call brought the daemon up on its way.
func daemonJustStarted(stderr string) bool {
	return strings.Contains(stderr, sbxDaemonStartingNotice)
}

// daemonWouldNotStart tells whether an sbx call failed because its daemon
// never came up.
func daemonWouldNotStart(stderr string) bool {
	return strings.Contains(stderr, sbxDaemonFailureNotice)
}
