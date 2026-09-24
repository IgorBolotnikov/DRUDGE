package cmd

import (
	"errors"
	"strings"
	"testing"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/project"
)

func TestProjectInit_NoArgs(t *testing.T) {
	if err := projectInit([]string{}); !errors.Is(err, ErrNoProjectName) {
		t.Fatalf("expected %v, got %v", ErrNoProjectName, err)
	}
}

func TestPrintRepositories(t *testing.T) {
	cases := []struct {
		name     string
		resolved []project.ResolvedRepository
		// wantLines are substrings the listing prints, in the order given.
		wantLines []string
	}{
		{
			name: "a repository that is the project directory",
			resolved: []project.ResolvedRepository{
				{Repository: config.Repository{Path: "."}, DefaultBranch: "main"},
			},
			wantLines: []string{"Repositories (1):", "REPOSITORY", "DEFAULT BRANCH", ".", "main"},
		},
		{
			name: "a repository whose default branch does not resolve",
			resolved: []project.ResolvedRepository{
				{Repository: config.Repository{Path: "api"}, DefaultBranch: "trunk"},
				{Repository: config.Repository{Path: "ui"}, Problem: errors.New("no origin/HEAD is set")},
			},
			wantLines: []string{"Repositories (2):", "api", "trunk", "ui", unresolvedBranch},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			log := common.NewLogger("")
			output := captureOutput(func() { printRepositories(log, testCase.resolved) })

			rest := output
			for _, want := range testCase.wantLines {
				index := strings.Index(rest, want)
				if index < 0 {
					t.Fatalf("expected the listing to hold %q after the lines before it, got:\n%s", want, output)
				}
				rest = rest[index+len(want):]
			}
		})
	}
}
