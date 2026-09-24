package cmd

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/project"
)

func TestRunProject_PrintsHelp(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantUsage string
	}{
		{name: "the project command", args: []string{helpFlag}, wantUsage: projectUsage},
		{name: "the project command with the short flag", args: []string{helpFlagShort}, wantUsage: projectUsage},
		{name: "create", args: []string{projectCreateSubcommand, helpFlag}, wantUsage: projectCreateUsage},
		{name: "init", args: []string{projectInitSubcommand, helpFlag}, wantUsage: projectInitUsage},
		{name: "init with the short flag", args: []string{projectInitSubcommand, helpFlagShort}, wantUsage: projectInitUsage},
		{name: "init with a name before the flag", args: []string{projectInitSubcommand, "demo", helpFlag}, wantUsage: projectInitUsage},
		{name: "delete", args: []string{projectDeleteSubcommand, helpFlag}, wantUsage: projectDeleteUsage},
		{name: "rename", args: []string{projectRenameSubcommand, helpFlag}, wantUsage: projectRenameUsage},
		{name: "list", args: []string{projectListSubcommand, helpFlag}, wantUsage: projectListUsage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var err error
			output := captureOutput(func() { err = runProject(testCase.args) })

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(output, testCase.wantUsage+"\n") {
				t.Errorf("expected the help to start with %q, got:\n%s", testCase.wantUsage, output)
			}
		})
	}
}

func TestRunProject_RefusesAMissingOrUnknownSubcommand(t *testing.T) {
	cases := []struct {
		name     string
		args     []string
		wantText string
	}{
		{name: "no subcommand", args: nil, wantText: projectUsage},
		{name: "an unknown subcommand", args: []string{"move"}, wantText: `unknown project subcommand "move"`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			err := runProject(testCase.args)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), testCase.wantText) {
				t.Errorf("expected the error to hold %q, got %q", testCase.wantText, err)
			}
		})
	}
}

func TestParseProjectArgs(t *testing.T) {
	cases := []struct {
		name           string
		args           []string
		argNames       []string
		isForceAllowed bool

		wantValues  []string
		wantForced  bool
		wantErrText string
	}{
		{
			name:       "a project name",
			args:       []string{"demo"},
			argNames:   []string{"project name"},
			wantValues: []string{"demo"},
		},
		{
			name:       "two names",
			args:       []string{"demo", "shop"},
			argNames:   []string{"old name", "new name"},
			wantValues: []string{"demo", "shop"},
		},
		{
			name:           "the force flag before the name",
			args:           []string{forceFlag, "demo"},
			argNames:       []string{"project name"},
			isForceAllowed: true,
			wantValues:     []string{"demo"},
			wantForced:     true,
		},
		{
			name:           "the short force flag",
			args:           []string{"demo", forceFlagShort},
			argNames:       []string{"project name"},
			isForceAllowed: true,
			wantValues:     []string{"demo"},
			wantForced:     true,
		},
		{
			name:        "the force flag where it is not taken",
			args:        []string{"demo", forceFlag},
			argNames:    []string{"project name"},
			wantErrText: `unknown flag "--force"`,
		},
		{
			name:        "an unknown flag",
			args:        []string{"--verbose", "demo"},
			argNames:    []string{"project name"},
			wantErrText: `unknown flag "--verbose"`,
		},
		{
			name:        "no name",
			args:        nil,
			argNames:    []string{"project name"},
			wantErrText: "project name is required",
		},
		{
			name:        "the second of two names missing",
			args:        []string{"demo"},
			argNames:    []string{"old name", "new name"},
			wantErrText: "new name is required",
		},
		{
			name:        "an extra argument",
			args:        []string{"demo", "shop"},
			argNames:    []string{"project name"},
			wantErrText: `unexpected argument "shop"`,
		},
		{
			name:        "an argument where none is taken",
			args:        []string{"demo"},
			wantErrText: `unexpected argument "demo"`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			const usage = "usage: drg project test"
			values, isForced, err := parseProjectArgs(testCase.args, testCase.argNames, testCase.isForceAllowed, usage)

			if testCase.wantErrText != "" {
				if err == nil {
					t.Fatalf("expected an error, got %v and force %v", values, isForced)
				}
				if !strings.Contains(err.Error(), testCase.wantErrText) || !strings.Contains(err.Error(), usage) {
					t.Errorf("expected the error to hold %q and the usage, got %q", testCase.wantErrText, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !slices.Equal(values, testCase.wantValues) {
				t.Errorf("expected %v, got %v", testCase.wantValues, values)
			}
			if isForced != testCase.wantForced {
				t.Errorf("expected force %v, got %v", testCase.wantForced, isForced)
			}
		})
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
