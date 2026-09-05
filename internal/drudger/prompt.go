package drudger

import (
	"fmt"
	"strings"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

// promptSourceDefault names the built-in template in error messages.
const promptSourceDefault = "built-in default prompt"

// Placeholders a prompt template may use.
const (
	placeholderTaskTitle       = "{{taskTitle}}"
	placeholderTaskDescription = "{{taskDescription}}"
	placeholderTicketID        = "{{ticketID}}"
	// TODO: pass a defualt branch as an optional placeholder
	placeholderDefaultBranch = "{{defaultBranch}}"
)

// requiredPlaceholders must be present in every prompt template.
var requiredPlaceholders = []string{placeholderTaskTitle, placeholderTaskDescription}

const defaultPromptTemplate = `Implement the following task end to end.

Ticket ID (if any): {{ticketID}}
Title: {{taskTitle}}

Description:
{{taskDescription}}

Work in the repository you were started in. Read the surrounding code before
you change it and follow the conventions you find there. Keep the change
scoped to this task.

You are working in a fully autonomous session. You will not get any
interactions from the user so use your best judgement when dealing with
uncertainty or problems. At the same time don't go down the rabbit hole of
fixing adjacent issues which don't directly block the ticket implementation.
 and leave unrelated problems you spot along the way alone.

Always follow established project code style and conventions.

Before you finish, always run self-review of the code you wrote and cleanup
any slop.

When you are done, make sure the project builds, all tests pass and the code
is formatted. Commit your work on a new branch and leave the review to a human.`

// renderPrompt fills a prompt template in with a task's details. A template
// missing a required placeholder is a hard error
func renderPrompt(template string, taskToRun *task.Task) (string, error) {
	for _, placeholder := range requiredPlaceholders {
		if !strings.Contains(template, placeholder) {
			return "", fmt.Errorf("prompt template is missing the required %s placeholder", placeholder)
		}
	}

	replacer := strings.NewReplacer(
		placeholderTaskTitle, taskToRun.Title,
		placeholderTaskDescription, taskToRun.Description,
		placeholderTicketID, taskToRun.TicketID,
	)
	return replacer.Replace(template), nil
}

// resolvePromptTemplate returns the prompt template to hand an agent, along
// with a description of where it came from for error messages. A prompt file
// named in either config wins over the built-in default.
func resolvePromptTemplate(local *config.LocalConfig, global *config.GlobalConfig) (template string, source string, err error) {
	path, err := config.ResolvePromptPath(local, global)
	if err != nil {
		return "", "", err
	}
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
	exists, err := common.Exists(path)
	if err != nil {
		return "", err
	}
	if !exists {
		return "", fmt.Errorf("prompt file %s does not exist", path)
	}

	return common.ReadFile(path)
}
