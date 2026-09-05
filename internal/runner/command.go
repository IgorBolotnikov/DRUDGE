package runner

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"drudge/internal/common"
	"drudge/internal/config"
)

type CommandRunner interface {
	Run(argv []string) (string, error)
}

// Pieces of an sbx invocation.
const (
	sbxBinary           = "sbx"
	sbxLsSubcommand     = "ls"
	sbxCreateSubcommand = "create"
	sbxExecSubcommand   = "exec"
	sbxJSONFlag         = "--json"
	sbxNameFlag         = "--name"
	sbxDetachedFlag     = "-d"

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

// Runner name prefixes.
const (
	claudeCodeRunnerPrefix = "drudge-claude"
	opencodeRunnerPrefix   = "drudge-opencode"
	unknownRunnerPrefix    = "drudge-unknown"
)

// unknownProjectSlug stands in for a project slug that normalizes to nothing.
const unknownProjectSlug = "unknown"

// sandboxPlan is the ordered set of commands that puts a runner to work.
type sandboxPlan struct {
	inspect []string // lists sandboxes so drudge can tell whether this runner's exists
	create  []string // creates it, run only when inspect says it is missing
	start   []string // starts the agent on the prompt, always run
}

// sandboxListing is what `sbx ls --json` reports.
type sandboxListing struct {
	Sandboxes []sandbox `json:"sandboxes"`
}

// sandbox is one entry of a sandbox listing.
type sandbox struct {
	Name string `json:"name"`
}

// pickRunnerCommand builds the commands that put an agent to work on the
// prompt sitting in the run directory.
func (service *RunnerService) pickRunnerCommand(projectSlug string, runnerID int, workspace, runDir string) (sandboxPlan, error) {
	env := service.globalCfg.Runner.Env
	harness := service.globalCfg.Runner.Harness
	name := formatRunnerName(projectSlug, runnerID, harness)

	if env == config.EnvDockerSbx && harness == config.HarnessClaudeCode {
		return sandboxPlan{
			inspect: []string{sbxBinary, sbxLsSubcommand, sbxJSONFlag},
			create:  []string{sbxBinary, sbxCreateSubcommand, sbxHarnessClaude, workspace, sbxNameFlag, name},
			start: []string{
				sbxBinary, sbxExecSubcommand, sbxDetachedFlag, name,
				shellBinary, shellCommandFlag, formatLauncher(workspace, runDir),
			},
		}, nil
	}

	return sandboxPlan{}, fmt.Errorf("DRUDGE does not know how to start harness %q in environment %q, check the runner settings in the config", harness, env)
}

// formatLauncher renders the shell script that runs the agent in a sandbox.
// The agent works in the workspace and its output lands in the run directory,
// so every path in it is absolute. The prompt is read from a file to keep an
// arbitrarily long, arbitrarily quoted prompt out of the command line.
//
// Permission prompts are bypassed because a headless agent stalling on a tool
// approval looks exactly like one that is stuck. The sandbox is the security
// boundary here.
//
// The exit file is written last and is the only sentinel of a finished run.
func formatLauncher(workspace, runDir string) string {
	agentCommand := strings.Join([]string{
		claudeBinary,
		claudePromptFlag, `"$(cat ` + shellQuote(common.RunPromptPath(runDir)) + `)"`,
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

// shellQuote wraps a value in single quotes so a shell reads it literally.
func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'\''`) + "'"
}

// sandboxExists reports whether a sandbox listing names the given sandbox.
func sandboxExists(listing string, name string) (bool, error) {
	var parsed sandboxListing
	if err := json.Unmarshal([]byte(listing), &parsed); err != nil {
		return false, fmt.Errorf("could not parse the sandbox listing: %w", err)
	}

	for _, candidate := range parsed.Sandboxes {
		if candidate.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// formatRunnerName names the sandbox a runner slot works in.
func formatRunnerName(projectSlug string, runnerID int, harness config.Harness) string {
	prefix := unknownRunnerPrefix
	switch harness {
	case config.HarnessClaudeCode:
		prefix = claudeCodeRunnerPrefix
	case config.HarnessOpencode:
		prefix = opencodeRunnerPrefix
	}
	return fmt.Sprintf("%s-%s-%d", prefix, normaliseNameSlug(projectSlug), runnerID)
}

// normaliseNameSlug normalizes the runner name to include only:
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

// formatArgv renders an argv for display.
func formatArgv(argv []string) string {
	quoted := make([]string, len(argv))
	for index, arg := range argv {
		quoted[index] = strconv.Quote(arg)
	}
	return strings.Join(quoted, " ")
}
