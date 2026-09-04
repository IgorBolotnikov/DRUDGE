package runner

import (
	"fmt"
	"strconv"
	"strings"

	"drudge/internal/config"
)

// CommandRunner runs an agent command. It is a port, so the runner service
// stays free of any process handling, and tests can hand it a fake.
type CommandRunner interface {
	// Run executes argv and returns what the command wrote to stdout.
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

// Runner name prefixes, one per harness.
const (
	claudeCodeRunnerPrefix = "drudge-claude"
	opencodeRunnerPrefix   = "drudge-opencode"
	unknownRunnerPrefix    = "drudge-unknown"
)

// pickRunnerCommand builds the argv that starts an agent on a prompt. The
// command is executed directly with no shell, so a multi-line prompt reaches
// the agent exactly as written.
func (service *RunnerService) pickRunnerCommand(runnerID int, prompt string) ([]string, error) {
	env := service.globalCfg.Runner.Env
	harness := service.globalCfg.Runner.Harness
	name := formatRunnerName(runnerID, harness)

	// Both harnesses take the same sbx invocation today and differ only in the
	// runner name. They stay separate branches so the one that changes first
	// can change on its own.
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

// formatArgv renders an argv for display. Every element is quoted, so a
// multi-line prompt stays readable as one argument.
func formatArgv(argv []string) string {
	quoted := make([]string, len(argv))
	for index, arg := range argv {
		quoted[index] = strconv.Quote(arg)
	}
	return strings.Join(quoted, " ")
}
