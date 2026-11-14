package task

import (
	"reflect"
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
