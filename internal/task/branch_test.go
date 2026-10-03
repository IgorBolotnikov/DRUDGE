package task

import (
	"strings"
	"testing"
)

func TestBranchName_DefaultFormat(t *testing.T) {
	cases := []struct {
		name  string
		id    TaskID
		title string
		want  string
	}{
		{name: "the short id and the title", id: "task-1", title: "Fix login", want: "drudge/task-1-fix-login"},
		{name: "a full uuid is cut to its leading characters", id: "a1b2c3d4-e5f6-7890-abcd-ef1234567890", title: "Fix login", want: "drudge/a1b2c3d4-fix-login"},
		{name: "punctuation a branch cannot carry is dropped", id: "task-1", title: "Fix: login/SSO (again)?", want: "drudge/task-1-fix-login-sso-again"},
		{name: "a title of nothing usable leaves the id alone", id: "task-1", title: "!?", want: "drudge/task-1"},
		// Thirteen words are the most that fit under the slug length.
		{name: "a long title keeps the words that fit", id: "task-1", title: strings.Repeat("ab ", 30), want: "drudge/task-1-" + strings.TrimSuffix(strings.Repeat("ab-", 13), "-")},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := BranchName(DefaultBranchFormat, &Task{ID: testCase.id, Title: testCase.title})
			if got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}
