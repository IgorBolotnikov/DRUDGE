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

func TestBranchName_ConfiguredFormat(t *testing.T) {
	cases := []struct {
		name     string
		format   string
		title    string
		ticketID string
		want     string
	}{
		{name: "literal text and the slug", format: "feat/" + BranchPlaceholderSlug, title: "Fix login", want: "feat/fix-login"},
		{name: "both placeholders in their own segments", format: "drg/" + BranchPlaceholderShortID + "/" + BranchPlaceholderSlug, title: "Fix login", want: "drg/task-1/fix-login"},
		{name: "an empty slug leaves no empty segment", format: "drudge/" + BranchPlaceholderSlug + "/drg-" + BranchPlaceholderShortID, title: "!?", want: "drudge/drg-task-1"},
		{name: "an empty slug leaves no trailing separator", format: "drudge/" + BranchPlaceholderShortID + "_" + BranchPlaceholderSlug, title: "!?", want: "drudge/task-1"},
		{name: "an empty slug leaves no leading separator", format: "drudge/" + BranchPlaceholderSlug + "." + BranchPlaceholderShortID, title: "!?", want: "drudge/task-1"},
		{name: "an empty slug alone in the name leaves the literal text", format: "/feat//" + BranchPlaceholderSlug + "/", title: "!?", want: "feat"},
		{name: "a ticket id keeps its case", format: "feature/" + BranchPlaceholderTicketID + "/drg-" + BranchPlaceholderSlug, title: "Fix login", ticketID: "ABC-123", want: "feature/ABC-123/drg-fix-login"},
		{name: "a ticket id loses what git refuses", format: "feature/" + BranchPlaceholderTicketID + "/drg-" + BranchPlaceholderSlug, title: "Fix login", ticketID: "Proj 7/a~b:c..D", want: "feature/Proj-7-a-b-c-D/drg-fix-login"},
		{name: "an empty ticket id in the middle", format: "feature/" + BranchPlaceholderTicketID + "/drg-" + BranchPlaceholderSlug, title: "Fix login", want: "feature/drg-fix-login"},
		{name: "an empty ticket id at the start", format: BranchPlaceholderTicketID + "/" + BranchPlaceholderSlug, title: "Fix login", want: "fix-login"},
		{name: "an empty ticket id at the end", format: BranchPlaceholderSlug + "-" + BranchPlaceholderTicketID, title: "Fix login", want: "fix-login"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := BranchName(testCase.format, &Task{ID: "task-1", Title: testCase.title, TicketID: testCase.ticketID})
			if got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestValidateBranchFormat(t *testing.T) {
	cases := []struct {
		name   string
		format string
		// wantErrText is a fragment the error must carry. A case with none expects no error.
		wantErrText string
	}{
		{name: "the default format", format: DefaultBranchFormat},
		{name: "literal text and both placeholders", format: "feat/drg-" + BranchPlaceholderShortID + "/" + BranchPlaceholderSlug},
		{name: "the slug alone", format: BranchPlaceholderSlug},
		{name: "the ticket id and the slug", format: "feature/" + BranchPlaceholderTicketID + "/drg-" + BranchPlaceholderSlug},
		{name: "the ticket id alone", format: "feature/" + BranchPlaceholderTicketID, wantErrText: BranchPlaceholderShortID + " or " + BranchPlaceholderSlug},
		{name: "no placeholder", format: "feat/fix", wantErrText: BranchPlaceholderShortID + " or " + BranchPlaceholderSlug},
		{name: "an unknown placeholder", format: "feat/{{tikcetID}}-" + BranchPlaceholderSlug, wantErrText: "{{tikcetID}}"},
		{name: "a name git refuses", format: "feat~" + BranchPlaceholderSlug, wantErrText: `"~"`},
		{name: "a segment ending with .lock", format: "feat.lock/" + BranchPlaceholderSlug, wantErrText: ".lock"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateBranchFormat(testCase.format)
			if testCase.wantErrText == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), testCase.wantErrText) {
				t.Errorf("error = %q, want it to name %q", err, testCase.wantErrText)
			}
		})
	}
}

func TestValidateBranchName(t *testing.T) {
	cases := []struct {
		name    string
		branch  string
		wantErr bool
	}{
		{name: "a plain name", branch: "drudge/3f9a-fix-login"},
		{name: "a name with dots inside segments", branch: "release/v1.2/fix"},
		{name: "empty", branch: "", wantErr: true},
		{name: "a space", branch: "drudge/fix login", wantErr: true},
		{name: "two dots", branch: "drudge/fix..login", wantErr: true},
		{name: "a tilde", branch: "drudge/fix~1", wantErr: true},
		{name: "a caret", branch: "drudge/fix^1", wantErr: true},
		{name: "a colon", branch: "drudge/fix:login", wantErr: true},
		{name: "a question mark", branch: "drudge/fix?", wantErr: true},
		{name: "an asterisk", branch: "drudge/fix*", wantErr: true},
		{name: "an open bracket", branch: "drudge/fix[1", wantErr: true},
		{name: "a backslash", branch: `drudge\fix`, wantErr: true},
		{name: "an at sign before a brace", branch: "drudge/fix@{1", wantErr: true},
		{name: "a control character", branch: "drudge/fix\tlogin", wantErr: true},
		{name: "a segment starting with a dot", branch: "drudge/.fix", wantErr: true},
		{name: "a segment ending with .lock", branch: "drudge.lock/fix", wantErr: true},
		{name: "a name ending with .lock", branch: "drudge/fix.lock", wantErr: true},
		{name: "a name ending with a dot", branch: "drudge/fix.", wantErr: true},
		{name: "a name ending with a slash", branch: "drudge/fix/", wantErr: true},
		{name: "a name starting with a slash", branch: "/drudge/fix", wantErr: true},
		{name: "two slashes in a row", branch: "drudge//fix", wantErr: true},
		{name: "a lone at sign", branch: "@", wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := ValidateBranchName(testCase.branch)
			if (err != nil) != testCase.wantErr {
				t.Errorf("ValidateBranchName(%q) = %v, want an error: %t", testCase.branch, err, testCase.wantErr)
			}
		})
	}
}
