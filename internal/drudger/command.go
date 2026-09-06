package drudger

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"drudge/internal/common"
	"drudge/internal/config"
)

// CommandRunner runs the commands that put a Drudger to work.
type CommandRunner interface {
	// Run waits for a command to finish and hands back what it wrote to
	// stdout and to stderr. Writing to stderr is not a failure on its own:
	// sbx reports its daemon there on calls that succeed.
	Run(argv []string) (stdout string, stderr string, err error)
	// Start runs the command and returns immediately.
	Start(argv []string) error
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
	// sbx asks for confirmation before removing a sandbox and this flag is used
	// only to skip the confirmation prompt. The flag is always passed since
	// drudge has no terminal to confirm the action. Drudge has its own
	// safety check for that.
	sbxForceFlag = "--force"

	sbxHarnessClaude = "claude"
	// This harness is to be implemented later
	sbxHarnessOpencode = "opencode"
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
)

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

// sandboxPlan is the ordered set of commands that puts a Drudger to work.
type sandboxPlan struct {
	inspect []string // lists sandboxes so drudge can tell whether this Drudger's sandbox exists
	create  []string // creates the sandbox, runs only when it does not exist yet
	start   []string // starts the agent and passes the prompt, always runs
}

// sandboxListing is what `sbx ls --json` reports.
type sandboxListing struct {
	Sandboxes []sandbox `json:"sandboxes"`
}

// sandbox is one entry of a sandbox listing.
type sandbox struct {
	Name       string   `json:"name"`
	Workspaces []string `json:"workspaces"`
}

// pickDrudgerCommand builds the commands that ensure the sandbox exists and
// put an agent to work on the configured prompt.
func (service *DrudgerService) pickDrudgerCommand(sandboxName string, workspace, runDir string) (sandboxPlan, error) {
	env := service.globalCfg.Drudger.Env
	harness := service.globalCfg.Drudger.Harness

	if env == config.EnvDockerSbx && harness == config.HarnessClaudeCode {
		return sandboxPlan{
			inspect: []string{sbxBinary, sbxLsSubcommand, sbxJSONFlag},
			create:  []string{sbxBinary, sbxCreateSubcommand, sbxHarnessClaude, workspace, sbxNameFlag, sandboxName},
			start: []string{
				sbxBinary, sbxExecSubcommand, sbxDetachedFlag, sandboxName,
				shellBinary, shellCommandFlag, formatLauncher(workspace, runDir),
			},
		}, nil
	}

	return sandboxPlan{}, fmt.Errorf("DRUDGE does not know how to start harness %q in environment %q, check the Drudger settings in the config", harness, env)
}

// pickRemoveCommand builds the command that deletes a Drudger's sandbox.
func (service *DrudgerService) pickRemoveCommand(sandboxName string) ([]string, error) {
	env := service.globalCfg.Drudger.Env

	if env == config.EnvDockerSbx {
		return []string{sbxBinary, sbxRmSubcommand, sbxForceFlag, sandboxName}, nil
	}

	return nil, fmt.Errorf("DRUDGE does not know how to remove a sandbox in environment %q, check the Drudger settings in the config", env)
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
// The exit file is written last and is the only marker of a finished run.
func formatLauncher(workspace, runDir string) string {
	agentCommand := strings.Join([]string{
		claudeBinary,
		claudePromptFlag, catPrompt(runDir),
		claudeOutputFormatFlag, claudeStreamJSONFormat,
		claudeVerboseFlag,
		claudePermissionFlag, claudeBypassPermissions,
	}, " ")

	return fmt.Sprintf(
		"cd %s || exit 1\n%s > %s 2> %s\necho $? > %s\n",
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

// checkSandboxWorkspace guards against an agent editing the wrong repository.
func checkSandboxWorkspace(existing *sandbox, workspace string) error {
	workspacePath := filepath.Clean(workspace)
	for _, mount := range existing.Workspaces {
		if filepath.Clean(mount) == workspacePath {
			return nil
		}
	}
	return fmt.Errorf(
		"sandbox %s is mounted on %s, but this project lives in %s, delete that sandbox so DRUDGE can recreate it on the right workspace",
		existing.Name, formatMounts(existing.Workspaces), workspace,
	)
}

// formatMounts renders a sandbox's workspace mounts for an error message.
func formatMounts(mounts []string) string {
	if len(mounts) == 0 {
		return "no workspace"
	}
	return strings.Join(mounts, ", ")
}

// formatDrudgerName names the sandbox a Drudger slot works in.
func formatDrudgerName(projectSlug string, drudgerSlot int, harness config.Harness) string {
	prefix := unknownDrudgerPrefix
	switch harness {
	case config.HarnessClaudeCode:
		prefix = claudeCodeDrudgerPrefix
	case config.HarnessOpencode:
		prefix = opencodeDrudgerPrefix
	}
	return fmt.Sprintf("%s-%s-%d", prefix, normaliseNameSlug(projectSlug), drudgerSlot)
}

// normaliseNameSlug normalizes the Drudger name to include only:
// lowercase letters, numbers, hyphens and periods. Anything else
// becomes a hyphen, and multiple consecutive hyphens collapse into one.
func normaliseNameSlug(projectSlug string) string {
	var name strings.Builder
	afterSeparator := false

	for _, char := range strings.ToLower(projectSlug) {
		if char >= 'a' && char <= 'z' || char >= '0' && char <= '9' || char == '.' {
			name.WriteRune(char)
			afterSeparator = false
			continue
		}
		if !afterSeparator {
			name.WriteByte('-')
			afterSeparator = true
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

// formatArgv renders an argv for display.
func formatArgv(argv []string) string {
	quoted := make([]string, len(argv))
	for index, arg := range argv {
		quoted[index] = strconv.Quote(arg)
	}
	return strings.Join(quoted, " ")
}
