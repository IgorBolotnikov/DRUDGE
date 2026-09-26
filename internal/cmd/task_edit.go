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
	title           optionalString
	description     optionalString
	descriptionFile optionalString
	ticket          optionalString
	status          optionalString
	blockedBy       optionalString
	block           optionalString
	unblock         optionalString
	parent          optionalString
	isForced        bool
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
	fs.BoolVar(&flags.isForced, forceFlagName, false, "Set a status drudge maintains itself ("+task.FormatStatuses(task.ManagedStatuses)+")")
	alias(fs, forceFlagShortName, forceFlagName)
}

// changes returns the fields an edit changes, reading a description file from
// disk or from stdin. It refuses more than one way to change the blockers and
// an edit that changes nothing.
func (flags *taskEditFlags) changes(stdin io.Reader) (task.EditTaskDto, error) {
	var givenBlockerFlags []string
	for _, blocker := range []struct {
		label string
		value optionalString
	}{
		{label: "--blocked-by", value: flags.blockedBy},
		{label: "--block", value: flags.block},
		{label: "--unblock", value: flags.unblock},
	} {
		if blocker.value.value != nil {
			givenBlockerFlags = append(givenBlockerFlags, blocker.label)
		}
	}
	if len(givenBlockerFlags) > 1 {
		return task.EditTaskDto{}, fmt.Errorf("%s cannot be used together, change the blockers one way per edit", common.JoinNames(givenBlockerFlags))
	}

	changes := task.EditTaskDto{
		Title:               optionalOf[string](flags.title),
		Description:         optionalOf[string](flags.description),
		TicketID:            optionalOf[string](flags.ticket),
		Status:              optionalOf[task.TaskStatus](flags.status),
		BlockedBy:           optionalTaskIDList(flags.blockedBy),
		Block:               optionalTaskIDList(flags.block),
		Unblock:             optionalTaskIDList(flags.unblock),
		ParentTaskID:        optionalOf[task.TaskID](flags.parent),
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

// taskEdit changes the fields a user owns on one task.
func taskEdit(taskID task.TaskID, changes task.EditTaskDto) error {
	deps, err := newCommandDeps()
	if err != nil {
		return err
	}

	_, err = deps.drudger.EditTask(deps.localCfg.ProjectSlug, taskID, changes)
	return err
}
