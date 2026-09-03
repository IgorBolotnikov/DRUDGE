package runner

import (
	"fmt"
	"strings"

	"drudge/internal/task"
)

// Placeholders a prompt template may use. Title and description are required
// in every template, the ticket ID is optional.
const (
	placeholderTaskTitle       = "{{taskTitle}}"
	placeholderTaskDescription = "{{taskDescription}}"
	placeholderTicketID        = "{{ticketID}}"
)

// requiredPlaceholders must be present in every prompt template, otherwise the
// agent would never learn what it is supposed to work on.
var requiredPlaceholders = []string{placeholderTaskTitle, placeholderTaskDescription}

// defaultPromptTemplate is handed to an agent when no prompt file is configured.
const defaultPromptTemplate = `Implement the following task end to end.

Ticket ID: {{ticketID}}
Title: {{taskTitle}}

Description:
{{taskDescription}}

Work in the repository you were started in. Read the surrounding code before you change it and follow the conventions you find there. Keep the change scoped to this task, and leave unrelated problems you spot along the way alone.

When you are done, make sure the project builds, all tests pass and the code is formatted. Commit your work on a new branch and leave the review to a human.`

// renderPrompt fills a prompt template in with a task's details. A template
// missing a required placeholder is an error naming the placeholder.
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
