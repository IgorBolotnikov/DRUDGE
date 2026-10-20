package project

import (
	"strings"
	"testing"
)

func TestProjectService_ResolveProject(t *testing.T) {
	existing := []*Project{{Name: "Drudge", Slug: "drudge"}, {Name: "Other", Slug: "other"}}

	cases := []struct {
		name         string
		explicitSlug string
		linkedSlug   string
		wantSlug     string
		// wantErrText is a fragment a refusal must carry. A case without it
		// expects a project.
		wantErrText string
	}{
		{
			name:         "a slug inside a project directory",
			explicitSlug: "other",
			linkedSlug:   "drudge",
			wantSlug:     "other",
		},
		{
			name:         "a slug outside a project directory",
			explicitSlug: "other",
			wantSlug:     "other",
		},
		{
			name:       "no slug inside a project directory",
			linkedSlug: "drudge",
			wantSlug:   "drudge",
		},
		{
			name:        "no slug outside a project directory",
			wantErrText: "pass -p <slug> or run from a project directory",
		},
		{
			name:         "a slug no project carries",
			explicitSlug: "ghost",
			linkedSlug:   "drudge",
			wantErrText:  `project "ghost" does not exist, run drg project init`,
		},
		{
			name:        "a project directory linked to a removed project",
			linkedSlug:  "ghost",
			wantErrText: `project "ghost" does not exist, run drg project init`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := NewProjectService(&fakeProjectRepo{projects: existing}, &fakeLinker{linkedSlug: testCase.linkedSlug}, nil, nil, &fakeProgress{})

			resolved, err := service.ResolveProject(testCase.explicitSlug)

			if testCase.wantErrText != "" {
				if err == nil || !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Fatalf("expected an error containing %q, got %v", testCase.wantErrText, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resolved.Slug != testCase.wantSlug {
				t.Errorf("expected project %s, got %s", testCase.wantSlug, resolved.Slug)
			}
		})
	}
}
