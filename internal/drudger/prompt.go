package drudger

import (
	"fmt"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/remote"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// promptSourceDefault names the built-in template in error messages.
const promptSourceDefault = "built-in default prompt"

// Placeholders a prompt template may use.
const (
	placeholderTaskTitle        = "{{taskTitle}}"
	placeholderTaskDescription  = "{{taskDescription}}"
	placeholderTicketID         = "{{ticketID}}"
	placeholderBranch           = "{{branch}}"
	placeholderDefaultBranch    = "{{defaultBranch}}"
	placeholderPullRequestSteps = "{{pullRequestSteps}}"
)

// Placeholders the pull request steps may use.
const (
	placeholderTitleFormat     = "{{titleFormat}}"
	placeholderTemplatePaths   = "{{templatePaths}}"
	placeholderDefaultTemplate = "{{defaultTemplate}}"
)

// requiredPlaceholders must be present in every prompt template.
var requiredPlaceholders = []string{placeholderTaskTitle, placeholderTaskDescription}

// pullRequestFilePath is where an agent writes the pull request description
// of a repository, relative to the root of its worktree.
const pullRequestFilePath = common.DotDrudgeDirName + "/pull-request.md"

// templatePathPrefix starts every template path the pull request steps list.
const templatePathPrefix = "- "

// promptWorkspace is what a prompt may say about the workspace a run happens
// in.
type promptWorkspace struct {
	// Branch is the branch housekeeping puts every repository on.
	Branch string
	// DefaultBranch names what the repositories cut work from.
	DefaultBranch string
}

// promptPullRequests is what a prompt says about pull requests. A prompt with
// pull requests on must take the steps.
type promptPullRequests struct {
	IsEnabled bool
	Steps     string
}

const defaultPromptTemplate = `Implement the following task end to end.

Ticket ID (if any): {{ticketID}}
Title: {{taskTitle}}

Description:
{{taskDescription}}

---
Work in the repository you were started in. Read the surrounding code before
you change it and follow the conventions you find there. Keep the change
scoped to this task.

You are working in a fully autonomous session. You will not get any
interactions from the user so use your best judgement when dealing with
uncertainty or problems. At the same time don't go down the rabbit hole of
fixing adjacent issues which don't directly block the ticket implementation
and leave unrelated problems you spot along the way alone.

Always follow established project code style and conventions.

Before you finish, always run self-review of the code you wrote and cleanup
any slop.

When you are done, make sure the project builds, all tests pass and the code
is formatted. The branch for this task, {{branch}}, is already checked out.
Commit your work on it. Do not create branches, do not switch branches and do
not push.

{{pullRequestSteps}}`

const defaultPullRequestSteps = `For every repository you committed to, write a pull request description to
` + pullRequestFilePath + ` at the root of that repository. Do not commit it.

The first line is the title, written as {{titleFormat}}. The rest is the body.

For the body use the pull request template of that repository. Look for it at
these paths, in this order, ignoring case:
{{templatePaths}}

If the repository has none, use the default template fenced between the <<<
and >>> lines below. Fill it in and leave the fence lines out.

<<<
{{defaultTemplate}}
>>>`

const defaultPullRequestTemplate = `## What

<what the change does>

## Why

<why it is needed>

## Testing

<how it was checked>`

// renderPrompt fills a prompt template in with a task's details, the
// workspace it runs in and the pull request steps. A template missing a
// required placeholder is a hard error.
func renderPrompt(template string, taskToRun *task.Task, workspace promptWorkspace, pullRequests promptPullRequests) (string, error) {
	for _, placeholder := range requiredPlaceholders {
		if !strings.Contains(template, placeholder) {
			return "", fmt.Errorf("prompt template is missing the required %s placeholder", placeholder)
		}
	}
	if pullRequests.IsEnabled && !strings.Contains(template, placeholderPullRequestSteps) {
		return "", fmt.Errorf("prompt template is missing the %s placeholder, which %s needs", placeholderPullRequestSteps, remote.PullRequestsEnabledKey)
	}

	replacer := strings.NewReplacer(
		placeholderTaskTitle, taskToRun.Title,
		placeholderTaskDescription, taskToRun.Description,
		placeholderTicketID, taskToRun.TicketID,
		placeholderBranch, workspace.Branch,
		placeholderDefaultBranch, workspace.DefaultBranch,
		placeholderPullRequestSteps, pullRequests.Steps,
	)
	return replacer.Replace(template), nil
}

// renderPullRequestSteps fills the pull request steps in. The title format
// gets the ticket ID and the title of the task, and is then handed over word
// for word.
func renderPullRequestSteps(steps string, titleFormat string, templatePaths []string, defaultTemplate string, taskToRun *task.Task) string {
	title := strings.NewReplacer(
		placeholderTicketID, taskToRun.TicketID,
		placeholderTaskTitle, taskToRun.Title,
	).Replace(titleFormat)

	listed := make([]string, 0, len(templatePaths))
	for _, path := range templatePaths {
		listed = append(listed, templatePathPrefix+path)
	}

	replacer := strings.NewReplacer(
		placeholderTitleFormat, title,
		placeholderTemplatePaths, strings.Join(listed, "\n"),
		placeholderDefaultTemplate, defaultTemplate,
	)
	return replacer.Replace(steps)
}

// pullRequestSteps renders the pull request steps of a task. With pull
// requests off there are none.
func (service *DrudgerService) pullRequestSteps(taskToRun *task.Task) (promptPullRequests, error) {
	if service.remote == nil {
		return promptPullRequests{}, nil
	}

	settings := service.settings.PullRequests
	steps, err := readPromptsFile(settings.StepsPath, defaultPullRequestSteps)
	if err != nil {
		return promptPullRequests{}, err
	}
	defaultTemplate, err := readPromptsFile(settings.TemplatePath, defaultPullRequestTemplate)
	if err != nil {
		return promptPullRequests{}, err
	}

	rendered := renderPullRequestSteps(steps, settings.TitleFormat, service.remote.TemplatePaths(), defaultTemplate, taskToRun)
	return promptPullRequests{IsEnabled: true, Steps: rendered}, nil
}

// readPromptsFile reads a file of the prompts directory without its trailing
// line breaks, which would otherwise add blank lines where it is filled in. An
// empty path returns fallback.
func readPromptsFile(path string, fallback string) (string, error) {
	if path == "" {
		return fallback, nil
	}
	content, err := loadPromptTemplate(path)
	if err != nil {
		return "", err
	}
	return strings.TrimRight(content, "\n"), nil
}

// resolvePromptTemplate returns the prompt template to hand an agent, along
// with a description of where it came from for error messages. An empty path
// means the built-in default.
func resolvePromptTemplate(path string) (template string, source string, err error) {
	if path == "" {
		return defaultPromptTemplate, promptSourceDefault, nil
	}

	templ, err := loadPromptTemplate(path)
	if err != nil {
		return "", "", err
	}
	return templ, path, nil
}

// loadPromptTemplate reads a prompt template from path.
func loadPromptTemplate(path string) (string, error) {
	isPresent, err := common.Exists(path)
	if err != nil {
		return "", err
	}
	if !isPresent {
		return "", fmt.Errorf("prompt file %s does not exist", path)
	}

	return common.ReadFile(path)
}
