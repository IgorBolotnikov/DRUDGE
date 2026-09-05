package runner

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"drudge/internal/config"
)

type CommandRunner interface {
	Run(argv []string) (string, error)
}

// Pieces of an sbx invocation.
const (
	sbxBinary        = "sbx"
	sbxRunSubcommand = "run"
	sbxNameFlag      = "--name"
	sbxDetachedFlag  = "--detached"
	sbxPromptFlag    = "-p"

	sbxHarnessClaude    = "claude"
	sbxHarnessOpencod   = "opencode"
	sbxCommandSeparator = "--"
)

// Runner name prefixes.
const (
	claudeCodeRunnerPrefix = "drudge-claude"
	opencodeRunnerPrefix   = "drudge-opencode"
	unknownRunnerPrefix    = "drudge-unknown"
)

// unknownProjectSlug stands in for a project slug that normalizes to nothing.
const unknownProjectSlug = "unknown"

// pickRunnerCommand builds the argv that starts an agent on a prompt.
func (service *RunnerService) pickRunnerCommand(projectSlug string, runnerID int, prompt string) ([]string, error) {
	env := service.globalCfg.Runner.Env
	harness := service.globalCfg.Runner.Harness
	name := formatRunnerName(projectSlug, runnerID, harness)

	switch env {
	case config.EnvDockerSbx:
		var sbxHarness string
		switch harness {
		case config.HarnessClaudeCode:
			sbxHarness = sbxHarnessClaude
		case config.HarnessOpencode:
			sbxHarness = sbxHarnessOpencod
		}

		if sbxHarness != "" {
			mainCommand := []string{sbxBinary, sbxRunSubcommand, sbxHarness, sbxNameFlag, name, sbxDetachedFlag}
			agentCommand := []string{sbxCommandSeparator, sbxPromptFlag, prompt}
			return slices.Concat(mainCommand, agentCommand), nil
		}
	}

	return nil, fmt.Errorf("DRUDGE does not know how to start harness %q in environment %q, check the runner settings in the config", harness, env)
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

// parseSessionID picks the session id a runner reports out of its output. The
// runner prints it on its last line. An empty result means the runner reported
// no session.
func parseSessionID(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range slices.Backward(lines) {
		line := strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
