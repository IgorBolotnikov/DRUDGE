package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
)

func TestRunProject_PrintsHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "the help flag", args: []string{ProjectCmd.Name, "--help"}},
		{name: "no subcommand", args: []string{ProjectCmd.Name}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var err error
			output := captureOutput(func() { err = NewRoot("v1.2.3").Execute(testCase.args) })

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(output, "usage: drg project <subcommand>\n") {
				t.Errorf("expected the help of the project command, got:\n%s", output)
			}
			for _, sub := range ProjectCmd.Subcommands {
				if !strings.Contains(output, "  "+sub.Name+" ") {
					t.Errorf("expected %q in the help, got:\n%s", sub.Name, output)
				}
			}
		})
	}
}

func TestRunProject_RefusesAnUnknownSubcommand(t *testing.T) {
	err := NewRoot("v1.2.3").Execute([]string{ProjectCmd.Name, "move"})

	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), `unknown subcommand "move"`) {
		t.Errorf("expected the error to name the unknown subcommand, got %q", err)
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

func TestProjectList_NoProjects(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	var err error
	output := captureOutput(func() { err = projectList(nil) })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "No projects yet") || !strings.Contains(output, "drg project init <name>") {
		t.Errorf("expected the listing to say there are no projects and how to create one, got:\n%s", output)
	}
}
