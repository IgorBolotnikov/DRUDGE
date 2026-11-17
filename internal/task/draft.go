package task

import (
	"errors"
	"fmt"
	"slices"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// Keys of the header of a draft.
const (
	draftTitleKey        = "title"
	draftStatusKey       = "status"
	draftTicketKey       = "ticket"
	draftParentKey       = "parent"
	draftBlockedByKey    = "blocked-by"
	draftReposKey        = "repos"
	draftPullRequestsKey = "pull-requests"
)

// draftKeys lists the keys of a draft in the order its header shows them.
var draftKeys = []string{draftTitleKey, draftStatusKey, draftTicketKey, draftParentKey, draftBlockedByKey, draftReposKey, draftPullRequestsKey}

// ListSeparator splits a list of task ids, URLs or repository paths.
const ListSeparator = ","

// listJoiner joins the entries of a list in a draft.
const listJoiner = ", "

// draftKeyJoiner joins the keys of a draft in an error.
const draftKeyJoiner = ", "

// draftDescriptionField names the description among the fields an edit from a
// draft changes.
const draftDescriptionField = "description"

// ErrDraftUnchanged says a draft describes no new task or changes nothing on
// its task, so no task was added or edited.
var ErrDraftUnchanged = errors.New("the draft is unchanged")

// DraftRemovalFailed reports a task added from a draft whose draft could not
// be removed. The task stands.
type DraftRemovalFailed struct {
	Task *Task
	Err  error
}

// EditDraftRemovalFailed reports a task edited from its draft whose draft could
// not be removed. The edit stands.
type EditDraftRemovalFailed struct {
	Task *Task
	Err  error
}

// Draft is a task written in an editor. It holds the fields of drg task
// new and the description.
type Draft struct {
	Title        string
	Status       TaskStatus
	TicketID     string
	ParentTaskID TaskID
	BlockedBy    []TaskID
	Repositories []string
	PullRequests []string
	Description  string
}

// FormatDraft writes a draft as a header with every key of a draft and the
// description as the body. Lists are comma-separated.
func FormatDraft(draft Draft) string {
	metadata := map[string]string{
		draftTitleKey:        draft.Title,
		draftStatusKey:       string(draft.Status),
		draftTicketKey:       draft.TicketID,
		draftParentKey:       string(draft.ParentTaskID),
		draftBlockedByKey:    joinList(draft.BlockedBy),
		draftReposKey:        joinList(draft.Repositories),
		draftPullRequestsKey: joinList(draft.PullRequests),
	}
	return common.FormatFrontMatterInOrder(metadata, draftKeys) + "\n" + draft.Description
}

// ParseDraft reads a draft FormatDraft wrote and the user edited. A key left
// out of the header reads as empty. It refuses a malformed header, an unknown
// key, a key given twice and an unknown status with an error that names the
// line.
func ParseDraft(data string) (Draft, error) {
	fields, description, err := common.ParseFrontMatterFields(data)
	if err != nil {
		return Draft{}, err
	}
	draft := Draft{Description: description}
	seenKeys := map[string]bool{}
	for _, field := range fields {
		if seenKeys[field.Key] {
			return Draft{}, fmt.Errorf("line %d: %s is set twice, keep one of the lines", field.LineNumber, field.Key)
		}
		seenKeys[field.Key] = true
		if err := draft.set(field); err != nil {
			return Draft{}, err
		}
	}
	return draft, nil
}

func (draft *Draft) set(field common.FrontMatterField) error {
	switch field.Key {
	case draftTitleKey:
		draft.Title = field.Value
	case draftStatusKey:
		status := TaskStatus(field.Value)
		if status != "" && !KnownStatus(status) {
			return fmt.Errorf("line %d: status %q is unknown, use one of: %s", field.LineNumber, status, FormatStatuses(Statuses))
		}
		draft.Status = status
	case draftTicketKey:
		draft.TicketID = field.Value
	case draftParentKey:
		draft.ParentTaskID = TaskID(field.Value)
	case draftBlockedByKey:
		draft.BlockedBy = ParseList[TaskID](field.Value)
	case draftReposKey:
		draft.Repositories = ParseList[string](field.Value)
	case draftPullRequestsKey:
		draft.PullRequests = ParseList[string](field.Value)
	default:
		return fmt.Errorf("line %d: a draft has no key %q, use one of: %s", field.LineNumber, field.Key, strings.Join(draftKeys, draftKeyJoiner))
	}
	return nil
}

// Dto returns the fields of the task a draft describes in a project. An empty
// status leaves the default status to the task service.
func (draft Draft) Dto(projectSlug string) CreateTaskDto {
	return CreateTaskDto{
		Title:        draft.Title,
		Description:  draft.Description,
		Status:       draft.Status,
		TicketID:     draft.TicketID,
		ParentTaskID: draft.ParentTaskID,
		ProjectSlug:  projectSlug,
		BlockedBy:    draft.BlockedBy,
		PullRequests: draft.PullRequests,
		Repositories: draft.Repositories,
	}
}

// taskDraft returns the draft that holds the current values of a task, with
// the description trimmed the way a parsed draft has it.
func taskDraft(stored *Task) Draft {
	return Draft{
		Title:        stored.Title,
		Status:       stored.Status,
		TicketID:     stored.TicketID,
		ParentTaskID: stored.ParentTaskID,
		BlockedBy:    stored.BlockedBy,
		Repositories: stored.Repositories,
		PullRequests: stored.PullRequests,
		Description:  strings.TrimSpace(stored.Description),
	}
}

// DiffDraft returns the edit that turns a task into what a draft of it
// describes. The edit names only the fields that differ, and a list that
// differs replaces the whole list. The blank space at the ends of the
// description of the task is ignored, because a parsed draft has none.
func DiffDraft(stored *Task, draft Draft) EditTaskDto {
	var changes EditTaskDto
	if draft.Title != stored.Title {
		changes.Title = &draft.Title
	}
	if draft.Status != stored.Status {
		changes.Status = &draft.Status
	}
	if draft.TicketID != stored.TicketID {
		changes.TicketID = &draft.TicketID
	}
	if draft.ParentTaskID != stored.ParentTaskID {
		changes.ParentTaskID = &draft.ParentTaskID
	}
	if !slices.Equal(draft.BlockedBy, stored.BlockedBy) {
		changes.BlockedBy = &draft.BlockedBy
	}
	if !slices.Equal(draft.Repositories, stored.Repositories) {
		changes.Repositories = &draft.Repositories
	}
	if !slices.Equal(draft.PullRequests, stored.PullRequests) {
		changes.PullRequests = &draft.PullRequests
	}
	if draft.Description != strings.TrimSpace(stored.Description) {
		changes.Description = &draft.Description
	}
	return changes
}

// DraftFields returns the names of the fields of a draft an edit changes, in
// the order a draft shows them.
func (changes EditTaskDto) DraftFields() []string {
	var fields []string
	appendIf := func(isChanged bool, field string) {
		if isChanged {
			fields = append(fields, field)
		}
	}
	appendIf(changes.Title != nil, draftTitleKey)
	appendIf(changes.Status != nil, draftStatusKey)
	appendIf(changes.TicketID != nil, draftTicketKey)
	appendIf(changes.ParentTaskID != nil, draftParentKey)
	appendIf(changes.BlockedBy != nil, draftBlockedByKey)
	appendIf(changes.Repositories != nil, draftReposKey)
	appendIf(changes.PullRequests != nil, draftPullRequestsKey)
	appendIf(changes.Description != nil, draftDescriptionField)
	return fields
}

// OpenDraft writes the template of a new task to the draft of a project
// unless the project keeps a draft, and returns the path of the draft. The
// template has the status a new task gets by default.
func (service *TaskService) OpenDraft(projectSlug string) (string, error) {
	return service.openDraft(projectSlug, NewTaskDraftName, service.draftTemplate())
}

// OpenTaskDraft writes the current values of a task to its draft unless the
// task keeps a draft, and returns the path of the draft. The id may be a
// prefix. Each task has a draft of its own, named after its full id.
func (service *TaskService) OpenTaskDraft(projectSlug string, id TaskID) (string, error) {
	if id == "" {
		return "", ErrNoTaskID
	}
	found, err := service.repo.FindTask(projectSlug, string(id))
	if err != nil {
		return "", err
	}
	return service.openDraft(projectSlug, taskDraftName(found.ID), FormatDraft(taskDraft(found)))
}

// ReadTaskDraft returns a task as it stands and the draft of it. The id may be
// a prefix. A missing or blank draft reads as the current values of the task.
// It refuses a draft that does not parse.
func (service *TaskService) ReadTaskDraft(projectSlug string, id TaskID) (*Task, Draft, error) {
	if id == "" {
		return nil, Draft{}, ErrNoTaskID
	}
	found, err := service.repo.FindTask(projectSlug, string(id))
	if err != nil {
		return nil, Draft{}, err
	}
	content, _, err := service.drafts.ReadDraft(projectSlug, taskDraftName(found.ID))
	if err != nil {
		return nil, Draft{}, err
	}
	if strings.TrimSpace(content) == "" {
		return found, taskDraft(found), nil
	}
	draft, err := ParseDraft(content)
	if err != nil {
		return nil, Draft{}, err
	}
	return found, draft, nil
}

// RemoveTaskDraft removes the draft of the task with the full id. A missing
// draft is no error.
func (service *TaskService) RemoveTaskDraft(projectSlug string, id TaskID) error {
	return service.drafts.RemoveDraft(projectSlug, taskDraftName(id))
}

func taskDraftName(id TaskID) string {
	return string(id)
}

// openDraft writes content to the draft called name unless the project keeps
// a draft by that name, and returns the path of the draft.
func (service *TaskService) openDraft(projectSlug string, name string, content string) (string, error) {
	_, isFound, err := service.drafts.ReadDraft(projectSlug, name)
	if err != nil {
		return "", err
	}
	if !isFound {
		if err := service.drafts.WriteDraft(projectSlug, name, content); err != nil {
			return "", err
		}
	}
	return service.drafts.DraftPath(projectSlug, name)
}

// CreateTaskFromDraft adds the task the draft of a project describes and
// removes the draft. A missing or empty draft, the template and a draft with
// no title and no description remove the draft and return ErrDraftUnchanged.
// A draft that does not parse or describes a task the service refuses stays,
// and the next open shows it again. A draft that cannot be removed once the
// task exists is reported as a warning event.
func (service *TaskService) CreateTaskFromDraft(projectSlug string) (*Task, error) {
	content, _, err := service.drafts.ReadDraft(projectSlug, NewTaskDraftName)
	if err != nil {
		return nil, err
	}
	if service.isBlankDraft(content) {
		return nil, service.cancelDraft(projectSlug)
	}
	draft, err := ParseDraft(content)
	if err != nil {
		return nil, err
	}
	if draft.Title == "" && draft.Description == "" {
		return nil, service.cancelDraft(projectSlug)
	}
	created, err := service.CreateTask(draft.Dto(projectSlug))
	if err != nil {
		return nil, err
	}
	if err := service.drafts.RemoveDraft(projectSlug, NewTaskDraftName); err != nil {
		service.progress.Report(DraftRemovalFailed{Task: created, Err: err})
	}
	return created, nil
}

func (service *TaskService) draftTemplate() string {
	return FormatDraft(Draft{Status: service.newTaskStatus()})
}

// isBlankDraft reports whether content is empty or the template, give or take
// the blank space an editor adds or strips at the ends.
func (service *TaskService) isBlankDraft(content string) bool {
	trimmed := strings.TrimSpace(content)
	return trimmed == "" || trimmed == strings.TrimSpace(service.draftTemplate())
}

// cancelDraft removes the draft of a new task and returns ErrDraftUnchanged,
// or the error the removal failed with.
func (service *TaskService) cancelDraft(projectSlug string) error {
	if err := service.drafts.RemoveDraft(projectSlug, NewTaskDraftName); err != nil {
		return err
	}
	return ErrDraftUnchanged
}

// ParseList reads a comma-separated list. Blank entries are dropped, so an
// empty value reads as an empty list.
func ParseList[Value ~string](value string) []Value {
	values := []Value{}
	for _, text := range strings.Split(value, ListSeparator) {
		if text = strings.TrimSpace(text); text != "" {
			values = append(values, Value(text))
		}
	}
	return values
}

func joinList[Value ~string](values []Value) string {
	texts := make([]string, 0, len(values))
	for _, value := range values {
		texts = append(texts, string(value))
	}
	return strings.Join(texts, listJoiner)
}
