package task

import (
	"errors"
	"reflect"
	"slices"
	"testing"
)

func TestFormatDraft(t *testing.T) {
	cases := []struct {
		name  string
		draft Draft
		want  string
	}{
		{
			name:  "empty draft lists every key",
			draft: Draft{},
			want:  "---\ntitle: \nstatus: \nticket: \nparent: \nblocked-by: \nrepos: \npull-requests: \n---\n\n",
		},
		{
			name: "every key, lists and a body",
			draft: Draft{
				Title:        "Add retry",
				Status:       StatusTodo,
				TicketID:     "UP-12",
				ParentTaskID: "3f9a1c2e",
				BlockedBy:    []TaskID{"a01f3b55", "7c1e4f90"},
				Repositories: []string{"api", "web"},
				PullRequests: []string{"https://github.com/acme/api/pull/7"},
				Description:  "Retry the upload.\n\nThree times.",
			},
			want: "---\ntitle: Add retry\nstatus: todo\nticket: UP-12\nparent: 3f9a1c2e\nblocked-by: a01f3b55, 7c1e4f90\nrepos: api, web\n" +
				"pull-requests: https://github.com/acme/api/pull/7\n---\n\nRetry the upload.\n\nThree times.",
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := FormatDraft(testCase.draft); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestParseDraft(t *testing.T) {
	cases := []struct {
		name    string
		data    string
		want    Draft
		wantErr string
	}{
		{name: "title", data: "---\ntitle: Add retry\n---\n", want: Draft{Title: "Add retry"}},
		{name: "status", data: "---\nstatus: draft\n---\n", want: Draft{Status: StatusDraft}},
		{name: "empty status", data: "---\nstatus:\n---\n", want: Draft{}},
		{name: "ticket", data: "---\nticket: UP-12\n---\n", want: Draft{TicketID: "UP-12"}},
		{name: "parent", data: "---\nparent: 3f9a1c2e\n---\n", want: Draft{ParentTaskID: "3f9a1c2e"}},
		{name: "blocked-by", data: "---\nblocked-by: a01f3b55,7c1e4f90\n---\n", want: Draft{BlockedBy: []TaskID{"a01f3b55", "7c1e4f90"}}},
		{name: "repos", data: "---\nrepos: api, web\n---\n", want: Draft{Repositories: []string{"api", "web"}}},
		{name: "pull-requests", data: "---\npull-requests: https://github.com/acme/api/pull/7\n---\n", want: Draft{PullRequests: []string{"https://github.com/acme/api/pull/7"}}},
		{name: "empty list", data: "---\nrepos: \n---\n", want: Draft{Repositories: []string{}}},
		{name: "list with blank entries", data: "---\nrepos: api, , web,\n---\n", want: Draft{Repositories: []string{"api", "web"}}},
		{name: "body", data: "---\ntitle: x\n---\n\nRetry the upload.\n\nThree times.\n", want: Draft{Title: "x", Description: "Retry the upload.\n\nThree times."}},
		{name: "empty body", data: "---\ntitle: x\n---\n\n\n", want: Draft{Title: "x"}},
		{name: "unknown key", data: "---\ntitle: x\ntilte: y\n---\n", wantErr: `line 3: a draft has no key "tilte", use one of: title, status, ticket, parent, blocked-by, repos, pull-requests`},
		{name: "key set twice", data: "---\ntitle: x\ntitle: y\n---\n", wantErr: "line 3: title is set twice, keep one of the lines"},
		{name: "unknown status", data: "---\nstatus: maybe\n---\n", wantErr: `line 2: status "maybe" is unknown, use one of: draft, todo, in-progress, fucked-up, unmerged, done`},
		{name: "malformed line", data: "---\ntitle x\n---\n", wantErr: `line 2: "title x" is not a key: value line, write it as key: value`},
		{name: "no header", data: "Retry the upload.\n", wantErr: "line 1: expected ---, start the file with a --- line above the fields"},
		{name: "unclosed header", data: "---\ntitle: x\n", wantErr: "line 3: the fields have no closing --- line, add one below the last field"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ParseDraft(testCase.data)
			if testCase.wantErr != "" {
				if err == nil || err.Error() != testCase.wantErr {
					t.Fatalf("expected error %q, got %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("expected %+v, got %+v", testCase.want, got)
			}
		})
	}
}

func TestParseDraft_RoundTrip(t *testing.T) {
	cases := []struct {
		name  string
		draft Draft
	}{
		{
			name:  "template",
			draft: Draft{Status: StatusTodo, BlockedBy: []TaskID{}, Repositories: []string{}, PullRequests: []string{}},
		},
		{
			name: "every key",
			draft: Draft{
				Title:        "Add retry: three times",
				Status:       StatusDraft,
				TicketID:     "UP-12",
				ParentTaskID: "3f9a1c2e",
				BlockedBy:    []TaskID{"a01f3b55", "7c1e4f90"},
				Repositories: []string{"api", "web"},
				PullRequests: []string{"https://github.com/acme/api/pull/7"},
				Description:  "Retry the upload.\n\n---\n\nThree times.",
			},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := ParseDraft(FormatDraft(testCase.draft))
			if err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			if !reflect.DeepEqual(got, testCase.draft) {
				t.Errorf("expected %+v, got %+v", testCase.draft, got)
			}
		})
	}
}

func TestDraft_Dto(t *testing.T) {
	draft := Draft{
		Title:        "Add retry",
		Status:       StatusTodo,
		TicketID:     "UP-12",
		ParentTaskID: "3f9a1c2e",
		BlockedBy:    []TaskID{"a01f3b55"},
		Repositories: []string{"api"},
		PullRequests: []string{"https://github.com/acme/api/pull/7"},
		Description:  "Retry the upload.",
	}
	want := CreateTaskDto{
		Title:        "Add retry",
		Description:  "Retry the upload.",
		Status:       StatusTodo,
		TicketID:     "UP-12",
		ParentTaskID: "3f9a1c2e",
		ProjectSlug:  "uploader",
		BlockedBy:    []TaskID{"a01f3b55"},
		PullRequests: []string{"https://github.com/acme/api/pull/7"},
		Repositories: []string{"api"},
	}
	if got := draft.Dto("uploader"); !reflect.DeepEqual(got, want) {
		t.Errorf("expected %+v, got %+v", want, got)
	}
}

const (
	draftTestProject = "uploader"
	draftTestPath    = "/home/user/.drudge/projects/uploader/drafts/new-task.md"
	draftTemplate    = "---\ntitle: \nstatus: draft\nticket: \nparent: \nblocked-by: \nrepos: \npull-requests: \n---\n\n"
	validDraft       = "---\ntitle: Add retry\nstatus: todo\n---\n\nRetry the upload.\n"
)

var errDraftRemoval = errors.New("could not remove the draft: permission denied")

// fakeDraftRepo keeps the drafts of one project in memory. removeErr fails
// every removal.
type fakeDraftRepo struct {
	drafts    map[string]string
	removeErr error
}

func newFakeDraftRepo(drafts map[string]string) *fakeDraftRepo {
	if drafts == nil {
		drafts = map[string]string{}
	}
	return &fakeDraftRepo{drafts: drafts}
}

func (repo *fakeDraftRepo) ReadDraft(projectSlug string, name string) (string, bool, error) {
	content, isFound := repo.drafts[name]
	return content, isFound, nil
}

func (repo *fakeDraftRepo) WriteDraft(projectSlug string, name string, content string) error {
	repo.drafts[name] = content
	return nil
}

func (repo *fakeDraftRepo) RemoveDraft(projectSlug string, name string) error {
	if repo.removeErr != nil {
		return repo.removeErr
	}
	delete(repo.drafts, name)
	return nil
}

func (repo *fakeDraftRepo) DraftPath(projectSlug string, name string) (string, error) {
	return draftTestPath, nil
}

func TestTaskService_OpenDraft(t *testing.T) {
	cases := []struct {
		name          string
		defaultStatus TaskStatus
		drafts        map[string]string
		wantDraft     string
	}{
		{name: "no draft gets the template", defaultStatus: StatusDraft, wantDraft: draftTemplate},
		{name: "the template falls back to todo", wantDraft: "---\ntitle: \nstatus: todo\nticket: \nparent: \nblocked-by: \nrepos: \npull-requests: \n---\n\n"},
		{name: "a kept draft is reopened", defaultStatus: StatusDraft, drafts: map[string]string{NewTaskDraftName: validDraft}, wantDraft: validDraft},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			drafts := newFakeDraftRepo(testCase.drafts)
			service := NewTaskService(&fakeTaskRepo{}, drafts, &fakeProgress{}, testCase.defaultStatus, nil)

			path, err := service.OpenDraft(draftTestProject)

			if err != nil {
				t.Fatalf("OpenDraft: %v", err)
			}
			if path != draftTestPath {
				t.Errorf("expected path %q, got %q", draftTestPath, path)
			}
			if got := drafts.drafts[NewTaskDraftName]; got != testCase.wantDraft {
				t.Errorf("expected draft %q, got %q", testCase.wantDraft, got)
			}
		})
	}
}

func TestTaskService_CreateTaskFromDraft(t *testing.T) {
	cases := []struct {
		name       string
		drafts     map[string]string
		removeErr  error
		wantTitles []string
		wantErr    error
		wantErrMsg string
		wantDraft  string
		// wantIsWarned says the failed removal is reported as a warning.
		wantIsWarned bool
	}{
		{name: "no draft cancels", wantErr: ErrDraftUnchanged},
		{name: "an empty draft cancels", drafts: map[string]string{NewTaskDraftName: " \n"}, wantErr: ErrDraftUnchanged},
		{name: "the template cancels", drafts: map[string]string{NewTaskDraftName: draftTemplate}, wantErr: ErrDraftUnchanged},
		{
			name:    "a template the editor trimmed cancels",
			drafts:  map[string]string{NewTaskDraftName: "---\ntitle:\nstatus: draft\nticket:\nparent:\nblocked-by:\nrepos:\npull-requests:\n---\n"},
			wantErr: ErrDraftUnchanged,
		},
		{name: "a draft with no title and no description cancels", drafts: map[string]string{NewTaskDraftName: "---\nstatus: todo\n---\n"}, wantErr: ErrDraftUnchanged},
		{
			name:      "a cancel that cannot remove the draft fails",
			drafts:    map[string]string{NewTaskDraftName: draftTemplate},
			removeErr: errDraftRemoval,
			wantErr:   errDraftRemoval,
			wantDraft: draftTemplate,
		},
		{
			name:       "a draft that does not parse stays",
			drafts:     map[string]string{NewTaskDraftName: "---\ntilte: Add retry\n---\n"},
			wantErrMsg: `line 2: a draft has no key "tilte", use one of: title, status, ticket, parent, blocked-by, repos, pull-requests`,
			wantDraft:  "---\ntilte: Add retry\n---\n",
		},
		{
			name:       "a draft the service refuses stays",
			drafts:     map[string]string{NewTaskDraftName: "---\ntitle: Add retry\nblocked-by: a01f3b55\n---\n"},
			wantErrMsg: `could not block on "a01f3b55": task "a01f3b55" not found`,
			wantDraft:  "---\ntitle: Add retry\nblocked-by: a01f3b55\n---\n",
		},
		{
			name:       "a created task removes the draft",
			drafts:     map[string]string{NewTaskDraftName: validDraft},
			wantTitles: []string{"Add retry"},
		},
		{
			name:         "a draft that cannot be removed after the create is a warning",
			drafts:       map[string]string{NewTaskDraftName: validDraft},
			removeErr:    errDraftRemoval,
			wantTitles:   []string{"Add retry"},
			wantDraft:    validDraft,
			wantIsWarned: true,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{}
			drafts := newFakeDraftRepo(testCase.drafts)
			drafts.removeErr = testCase.removeErr
			progress := &fakeProgress{}
			service := NewTaskService(repo, drafts, progress, StatusDraft, nil)

			created, err := service.CreateTaskFromDraft(draftTestProject)

			switch {
			case testCase.wantErrMsg != "":
				if err == nil || err.Error() != testCase.wantErrMsg {
					t.Errorf("expected error %q, got %v", testCase.wantErrMsg, err)
				}
			case !errors.Is(err, testCase.wantErr):
				t.Errorf("expected error %v, got %v", testCase.wantErr, err)
			}
			var gotTitles []string
			for _, stored := range repo.tasks {
				gotTitles = append(gotTitles, stored.Title)
			}
			if !slices.Equal(gotTitles, testCase.wantTitles) {
				t.Errorf("expected tasks %q, got %q", testCase.wantTitles, gotTitles)
			}
			if wantIsReturned := len(testCase.wantTitles) > 0; wantIsReturned != (created != nil) {
				t.Errorf("expected the created task back: %v, got %+v", wantIsReturned, created)
			}
			if got := drafts.drafts[NewTaskDraftName]; got != testCase.wantDraft {
				t.Errorf("expected draft %q, got %q", testCase.wantDraft, got)
			}
			var wantWarnings []any
			if testCase.wantIsWarned {
				wantWarnings = []any{DraftRemovalFailed{Task: created, Err: errDraftRemoval}}
			}
			if got := draftRemovalWarnings(progress.events); !reflect.DeepEqual(got, wantWarnings) {
				t.Errorf("expected warnings %+v, got %+v", wantWarnings, got)
			}
		})
	}
}

func draftRemovalWarnings(events []any) []any {
	var warnings []any
	for _, event := range events {
		if _, ok := event.(DraftRemovalFailed); ok {
			warnings = append(warnings, event)
		}
	}
	return warnings
}

func TestDiffDraft(t *testing.T) {
	stored := &Task{
		ID:           "3f9a1c2e-0000-4000-8000-000000000001",
		Title:        "Add retry",
		Status:       StatusTodo,
		TicketID:     "UP-12",
		ParentTaskID: "5b20d7aa",
		BlockedBy:    []TaskID{"a01f3b55"},
		Repositories: []string{"api"},
		PullRequests: []string{"https://github.com/acme/api/pull/7"},
		Description:  "Retry the upload.\n",
	}
	unchanged := func() Draft {
		return Draft{
			Title:        "Add retry",
			Status:       StatusTodo,
			TicketID:     "UP-12",
			ParentTaskID: "5b20d7aa",
			BlockedBy:    []TaskID{"a01f3b55"},
			Repositories: []string{"api"},
			PullRequests: []string{"https://github.com/acme/api/pull/7"},
			Description:  "Retry the upload.",
		}
	}
	title := "Add retry to uploader"
	status := StatusDraft
	ticket := "UP-13"
	parent := TaskID("")
	blockedBy := []TaskID{"a01f3b55", "7c1e4f90"}
	repositories := []string{"api", "web"}
	noPullRequests := []string{}
	description := "Retry the upload three times."
	cases := []struct {
		name       string
		edit       func(draft *Draft)
		want       EditTaskDto
		wantFields []string
	}{
		{name: "nothing changed", edit: func(draft *Draft) {}},
		{name: "title", edit: func(draft *Draft) { draft.Title = title }, want: EditTaskDto{Title: &title}, wantFields: []string{"title"}},
		{name: "status", edit: func(draft *Draft) { draft.Status = status }, want: EditTaskDto{Status: &status}, wantFields: []string{"status"}},
		{name: "ticket", edit: func(draft *Draft) { draft.TicketID = ticket }, want: EditTaskDto{TicketID: &ticket}, wantFields: []string{"ticket"}},
		{name: "parent cleared", edit: func(draft *Draft) { draft.ParentTaskID = parent }, want: EditTaskDto{ParentTaskID: &parent}, wantFields: []string{"parent"}},
		{name: "blockers replaced", edit: func(draft *Draft) { draft.BlockedBy = blockedBy }, want: EditTaskDto{BlockedBy: &blockedBy}, wantFields: []string{"blocked-by"}},
		{name: "repositories replaced", edit: func(draft *Draft) { draft.Repositories = repositories }, want: EditTaskDto{Repositories: &repositories}, wantFields: []string{"repos"}},
		{name: "pull requests cleared", edit: func(draft *Draft) { draft.PullRequests = noPullRequests }, want: EditTaskDto{PullRequests: &noPullRequests}, wantFields: []string{"pull-requests"}},
		{name: "description", edit: func(draft *Draft) { draft.Description = description }, want: EditTaskDto{Description: &description}, wantFields: []string{"description"}},
		{
			name:       "fields in the order of a draft",
			edit:       func(draft *Draft) { draft.Description, draft.Title = description, title },
			want:       EditTaskDto{Title: &title, Description: &description},
			wantFields: []string{"title", "description"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			draft := unchanged()
			testCase.edit(&draft)

			got := DiffDraft(stored, draft)

			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("expected %+v, got %+v", testCase.want, got)
			}
			if fields := got.DraftFields(); !slices.Equal(fields, testCase.wantFields) {
				t.Errorf("expected fields %q, got %q", testCase.wantFields, fields)
			}
		})
	}
}

const (
	draftTestTaskID TaskID = "3f9a1c2e-0000-4000-8000-000000000001"
	storedTaskDraft        = "---\ntitle: Add retry\nstatus: todo\nticket: UP-12\nparent: \nblocked-by: \nrepos: api\npull-requests: \n---\n\nRetry the upload."
	keptTaskDraft          = "---\ntitle: Add retry to uploader\n---\n"
)

func TestTaskService_OpenTaskDraft(t *testing.T) {
	cases := []struct {
		name      string
		id        TaskID
		drafts    map[string]string
		wantErr   string
		wantDraft string
	}{
		{name: "no draft gets the values of the task", id: draftTestTaskID, wantDraft: storedTaskDraft},
		{name: "a prefix names the task", id: "3f9a", wantDraft: storedTaskDraft},
		{name: "a kept draft is reopened", id: draftTestTaskID, drafts: map[string]string{string(draftTestTaskID): keptTaskDraft}, wantDraft: keptTaskDraft},
		{name: "the draft of a new task is left alone", id: draftTestTaskID, drafts: map[string]string{NewTaskDraftName: validDraft}, wantDraft: storedTaskDraft},
		{name: "an unknown task opens no draft", id: "ffff", wantErr: `task "ffff" not found`},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{tasks: []*Task{draftTestTask()}}
			drafts := newFakeDraftRepo(testCase.drafts)
			service := NewTaskService(repo, drafts, &fakeProgress{}, StatusDraft, nil)

			path, err := service.OpenTaskDraft(draftTestProject, testCase.id)

			if testCase.wantErr != "" {
				if err == nil || err.Error() != testCase.wantErr {
					t.Fatalf("expected error %q, got %v", testCase.wantErr, err)
				}
				if len(drafts.drafts) != len(testCase.drafts) {
					t.Errorf("expected no draft written, got %q", drafts.drafts)
				}
				return
			}
			if err != nil {
				t.Fatalf("OpenTaskDraft: %v", err)
			}
			if path != draftTestPath {
				t.Errorf("expected path %q, got %q", draftTestPath, path)
			}
			if got := drafts.drafts[string(draftTestTaskID)]; got != testCase.wantDraft {
				t.Errorf("expected draft %q, got %q", testCase.wantDraft, got)
			}
		})
	}
}

func TestTaskService_ReadTaskDraft(t *testing.T) {
	storedDraft := Draft{Title: "Add retry", Status: StatusTodo, TicketID: "UP-12", Repositories: []string{"api"}, Description: "Retry the upload."}
	cases := []struct {
		name      string
		drafts    map[string]string
		wantDraft Draft
		wantErr   string
	}{
		{name: "no draft reads as the task", wantDraft: storedDraft},
		{name: "a blank draft reads as the task", drafts: map[string]string{string(draftTestTaskID): " \n"}, wantDraft: storedDraft},
		{name: "an edited draft", drafts: map[string]string{string(draftTestTaskID): keptTaskDraft}, wantDraft: Draft{Title: "Add retry to uploader"}},
		{
			name:    "a draft that does not parse",
			drafts:  map[string]string{string(draftTestTaskID): "---\ntilte: Add retry\n---\n"},
			wantErr: `line 2: a draft has no key "tilte", use one of: title, status, ticket, parent, blocked-by, repos, pull-requests`,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			repo := &fakeTaskRepo{tasks: []*Task{draftTestTask()}}
			service := NewTaskService(repo, newFakeDraftRepo(testCase.drafts), &fakeProgress{}, StatusDraft, nil)

			stored, draft, err := service.ReadTaskDraft(draftTestProject, "3f9a")

			if testCase.wantErr != "" {
				if err == nil || err.Error() != testCase.wantErr {
					t.Fatalf("expected error %q, got %v", testCase.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ReadTaskDraft: %v", err)
			}
			if stored.ID != draftTestTaskID {
				t.Errorf("expected task %s, got %s", draftTestTaskID, stored.ID)
			}
			if !reflect.DeepEqual(draft, testCase.wantDraft) {
				t.Errorf("expected draft %+v, got %+v", testCase.wantDraft, draft)
			}
		})
	}
}

func draftTestTask() *Task {
	return &Task{ID: draftTestTaskID, Title: "Add retry", Status: StatusTodo, TicketID: "UP-12", Repositories: []string{"api"}, Description: "Retry the upload.\n"}
}
