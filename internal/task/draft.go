package task

import (
	"fmt"
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

// Draft is a new task written in an editor. It holds the fields of drg task
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
