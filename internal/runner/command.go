package runner

import (
	"fmt"
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
)

// Runner name prefixes.
const (
	claudeCodeRunnerPrefix = "drudge-claude"
	opencodeRunnerPrefix   = "drudge-opencode"
	unknownRunnerPrefix    = "drudge-unknown"
)

// pickRunnerCommand builds the argv that starts an agent on a prompt.
func (service *RunnerService) pickRunnerCommand(runnerID int, prompt string) ([]string, error) {
	env := service.globalCfg.Runner.Env
	harness := service.globalCfg.Runner.Harness
	name := formatRunnerName(runnerID, harness)

	switch env {
	case config.EnvDockerSbx:
		switch harness {
		case config.HarnessClaudeCode:
			return []string{sbxBinary, sbxRunSubcommand, sbxNameFlag, name, sbxDetachedFlag, sbxPromptFlag, prompt}, nil
		case config.HarnessOpencode:
			return []string{sbxBinary, sbxRunSubcommand, sbxNameFlag, name, sbxDetachedFlag, sbxPromptFlag, prompt}, nil
		}
	}

	return nil, fmt.Errorf("DRUDGE does not know how to start harness %q in environment %q, check the runner settings in the config", harness, env)
}

func formatRunnerName(runnerID int, harness config.Harness) string {
	prefix := unknownRunnerPrefix
	switch harness {
	case config.HarnessClaudeCode:
		prefix = claudeCodeRunnerPrefix
	case config.HarnessOpencode:
		prefix = opencodeRunnerPrefix
	}
	return fmt.Sprintf("%s-%d", prefix, runnerID)
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
// runner prints it on its last line, so anything it logged before is skipped.
// An empty result means the runner reported no session at all.
func parseSessionID(output string) string {
	lines := strings.Split(output, "\n")
	for index := len(lines) - 1; index >= 0; index-- {
		line := strings.TrimSpace(lines[index])
		if line != "" {
			return line
		}
	}
	return ""
}
