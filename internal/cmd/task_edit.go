package cmd

import (
	"flag"
	"fmt"
	"io"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// taskEditFlags holds the flags of drg task edit. A flag left out leaves its
// field as it stands, and a flag given an empty value clears its field.
type taskEditFlags struct {
	title             optionalString
	description       optionalString
	descriptionFile   optionalString
	ticket            optionalString
	status            optionalString
	blockedBy         optionalString
	block             optionalString
	unblock           optionalString
	parent            optionalString
	pullRequests      optionalString
	addPullRequest    optionalString
	removePullRequest optionalString
	isForced          bool
}

func (flags *taskEditFlags) declare(fs *flag.FlagSet) {
	fs.Var(&flags.title, titleFlagName, "New `title`")
	fs.Var(&flags.description, descriptionFlagName, "New description, the `text` the agent is handed as its prompt")
	fs.Var(&flags.descriptionFile, descriptionFileFlagName, "Read the new description from the file at `path`, "+stdinPath+" to read stdin")
	fs.Var(&flags.ticket, ticketFlagName, "The `ticket` the task came from, empty to clear it")
	fs.Var(&flags.status, statusFlagName, "New `status` ("+task.FormatStatuses(task.Statuses)+")")
	fs.Var(&flags.blockedBy, blockedByFlagName, "Comma-separated `ids` of the tasks this task waits for, replacing the list, empty to clear it")
	fs.Var(&flags.block, blockFlagName, "Comma-separated `ids` of tasks to add to the ones this task waits for")
	fs.Var(&flags.unblock, unblockFlagName, "Comma-separated `ids` of tasks to remove from the ones this task waits for")
	fs.Var(&flags.parent, parentFlagName, "The `id` of the task this task belongs to, empty to ungroup it")
	fs.Var(&flags.pullRequests, pullRequestsFlagName, "Comma-separated `urls` of the pull requests opened for this task, replacing the list, empty to clear it")
	fs.Var(&flags.addPullRequest, addPullRequestFlagName, "Comma-separated `urls` of pull requests to add to the ones opened for this task")
	fs.Var(&flags.removePullRequest, removePullRequestFlagName, "Comma-separated `urls` of pull requests to remove from the ones opened for this task")
	fs.BoolVar(&flags.isForced, forceFlagName, false, "Set a status drudge maintains itself ("+task.FormatStatuses(task.ManagedStatuses)+")")
	alias(fs, forceFlagShortName, forceFlagName)
}

// changes returns the fields an edit changes, reading a description file from
// disk or from stdin. It refuses more than one way to change the blockers or
// the pull requests, and an edit that changes nothing.
func (flags *taskEditFlags) changes(stdin io.Reader) (task.EditTaskDto, error) {
	if err := refuseTogether("the blockers", []namedFlag{
		{name: blockedByFlagName, value: flags.blockedBy},
		{name: blockFlagName, value: flags.block},
		{name: unblockFlagName, value: flags.unblock},
	}); err != nil {
		return task.EditTaskDto{}, err
	}
	if err := refuseTogether("the pull requests", []namedFlag{
		{name: pullRequestsFlagName, value: flags.pullRequests},
		{name: addPullRequestFlagName, value: flags.addPullRequest},
		{name: removePullRequestFlagName, value: flags.removePullRequest},
	}); err != nil {
		return task.EditTaskDto{}, err
	}

	changes := task.EditTaskDto{
		Title:               optionalOf[string](flags.title),
		Description:         optionalOf[string](flags.description),
		TicketID:            optionalOf[string](flags.ticket),
		Status:              optionalOf[task.TaskStatus](flags.status),
		BlockedBy:           optionalList[task.TaskID](flags.blockedBy),
		Block:               optionalList[task.TaskID](flags.block),
		Unblock:             optionalList[task.TaskID](flags.unblock),
		ParentTaskID:        optionalOf[task.TaskID](flags.parent),
		PullRequests:        optionalList[string](flags.pullRequests),
		AddPullRequests:     optionalList[string](flags.addPullRequest),
		RemovePullRequests:  optionalList[string](flags.removePullRequest),
		AllowsManagedStatus: flags.isForced,
	}
	if flags.descriptionFile.value != nil {
		if changes.Description != nil {
			return task.EditTaskDto{}, errTwoDescriptions
		}
		description, err := readDescriptionFile(flags.descriptionFile.get(), stdin)
		if err != nil {
			return task.EditTaskDto{}, err
		}
		changes.Description = &description
	}
	if !changes.HasChanges() {
		return task.EditTaskDto{}, task.ErrNoChanges
	}
	return changes, nil
}

// namedFlag is a flag of an edit and the value it was given.
type namedFlag struct {
	name  string
	value optionalString
}

// refuseTogether refuses more than one of the flags that change a field in
// different ways.
func refuseTogether(field string, flags []namedFlag) error {
	var given []string
	for _, candidate := range flags {
		if candidate.value.value != nil {
			given = append(given, flagLabel(candidate.name))
		}
	}
	if len(given) > 1 {
		return fmt.Errorf("%s cannot be used together, change %s one way per edit", common.JoinNames(given), field)
	}
	return nil
}

// taskEdit changes the fields a user owns on one task.
func taskEdit(taskID task.TaskID, changes task.EditTaskDto) error {
	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	_, err = deps.drudger.EditTask(deps.localCfg.ProjectSlug, taskID, changes)
	return err
}
