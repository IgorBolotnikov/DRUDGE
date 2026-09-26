package cmd

import (
	"flag"
	"slices"
	"strings"
	"testing"
)

// invocation records one call of a run function of the fake tree.
type invocation struct {
	command string
	args    []string
	isForce bool
	title   string
	ticket  *string
}

// newFakeTree builds a tree of a root with a --verbose flag, a task group
// holding the rm and list leaves. Every run function appends what it got to
// calls.
func newFakeTree(calls *[]invocation) *Cmd {
	return &Cmd{
		Name: "app",
		Desc: "Run the app",
		Setup: func(fs *flag.FlagSet) func(args []string) error {
			fs.Bool("verbose", false, "Print more")
			return func(args []string) error { return flag.ErrHelp }
		},
		Subcommands: []*Cmd{
			{
				Name: "task",
				Desc: "Manage tasks",
				Subcommands: []*Cmd{
					{
						Name: "rm",
						Args: []string{"task-id"},
						Desc: "Delete a task",
						Help: "Delete a task by its id.",
						Setup: func(fs *flag.FlagSet) func(args []string) error {
							isForce := fs.Bool("force", false, "Skip the confirmation")
							alias(fs, "f", "force")
							title := fs.String("title", "", "Delete only when the task has this `title`")
							ticket := &optionalString{}
							fs.Var(ticket, "ticket", "Delete only when the task has this `ticket`, or none when empty")
							return func(args []string) error {
								*calls = append(*calls, invocation{command: "rm", args: args, isForce: *isForce, title: *title, ticket: ticket.value})
								return nil
							}
						},
					},
					{
						Name: "list",
						Desc: "List the tasks",
						Setup: func(fs *flag.FlagSet) func(args []string) error {
							return func(args []string) error {
								*calls = append(*calls, invocation{command: "list", args: args})
								return nil
							}
						},
					},
				},
			},
		},
	}
}

func stringPointer(value string) *string {
	return &value
}

func TestExecute_Dispatches(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want invocation
	}{
		{
			name: "a leaf of a group",
			args: []string{"task", "list"},
			want: invocation{command: "list"},
		},
		{
			name: "a leaf with a positional",
			args: []string{"task", "rm", "42"},
			want: invocation{command: "rm", args: []string{"42"}},
		},
		{
			name: "flags before the positional",
			args: []string{"task", "rm", "--force", "--title", "Wire it", "42"},
			want: invocation{command: "rm", args: []string{"42"}, isForce: true, title: "Wire it"},
		},
		{
			name: "flags after the positional",
			args: []string{"task", "rm", "42", "-f", "-title=Wire it"},
			want: invocation{command: "rm", args: []string{"42"}, isForce: true, title: "Wire it"},
		},
		{
			name: "root flags before the subcommand",
			args: []string{"--verbose", "task", "rm", "42"},
			want: invocation{command: "rm", args: []string{"42"}},
		},
		{
			name: "flag-like positionals after the terminator",
			args: []string{"task", "rm", "--title", "x", "--", "--force"},
			want: invocation{command: "rm", args: []string{"--force"}, title: "x"},
		},
		{
			name: "a terminator after the positional",
			args: []string{"task", "rm", "42", "--"},
			want: invocation{command: "rm", args: []string{"42"}},
		},
		{
			name: "an optional flag given empty",
			args: []string{"task", "rm", "42", "--ticket="},
			want: invocation{command: "rm", args: []string{"42"}, ticket: stringPointer("")},
		},
		{
			name: "an optional flag given a value",
			args: []string{"task", "rm", "42", "--ticket", "R-7"},
			want: invocation{command: "rm", args: []string{"42"}, ticket: stringPointer("R-7")},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls []invocation
			root := newFakeTree(&calls)

			err := root.Execute(testCase.args)

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(calls) != 1 {
				t.Fatalf("expected one call, got %+v", calls)
			}
			got := calls[0]
			if got.command != testCase.want.command || !slices.Equal(got.args, testCase.want.args) ||
				got.isForce != testCase.want.isForce || got.title != testCase.want.title {
				t.Errorf("expected %+v, got %+v", testCase.want, got)
			}
			if (got.ticket == nil) != (testCase.want.ticket == nil) ||
				(got.ticket != nil && *got.ticket != *testCase.want.ticket) {
				t.Errorf("expected ticket %v, got %v", testCase.want.ticket, got.ticket)
			}
		})
	}
}

func TestExecute_PrintsHelp(t *testing.T) {
	const (
		rootUsage  = "usage: app <subcommand>\n"
		groupUsage = "usage: app task <subcommand>\n"
		leafUsage  = "usage: app task rm <task-id> [options]\n"
		plainUsage = "usage: app task list\n"
	)
	cases := []struct {
		name      string
		args      []string
		wantUsage string
	}{
		{name: "the root with no args", args: nil, wantUsage: rootUsage},
		{name: "the root with -h", args: []string{"-h"}, wantUsage: rootUsage},
		{name: "the root with -help", args: []string{"-help"}, wantUsage: rootUsage},
		{name: "the root with --help", args: []string{"--help"}, wantUsage: rootUsage},
		{name: "a bare group", args: []string{"task"}, wantUsage: groupUsage},
		{name: "a group with -h", args: []string{"task", "-h"}, wantUsage: groupUsage},
		{name: "a group with -help", args: []string{"task", "-help"}, wantUsage: groupUsage},
		{name: "a group with --help", args: []string{"task", "--help"}, wantUsage: groupUsage},
		{name: "a leaf with -h", args: []string{"task", "rm", "-h"}, wantUsage: leafUsage},
		{name: "a leaf with -help", args: []string{"task", "rm", "-help"}, wantUsage: leafUsage},
		{name: "a leaf with --help after the positional", args: []string{"task", "rm", "42", "--help"}, wantUsage: leafUsage},
		{name: "a leaf without flags", args: []string{"task", "list", "--help"}, wantUsage: plainUsage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls []invocation
			root := newFakeTree(&calls)

			var err error
			output := captureOutput(func() { err = root.Execute(testCase.args) })

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(calls) != 0 {
				t.Errorf("expected no run function called, got %+v", calls)
			}
			if !strings.HasPrefix(output, testCase.wantUsage) {
				t.Errorf("expected the help to start with %q, got:\n%s", testCase.wantUsage, output)
			}
		})
	}
}

func TestExecute_RendersHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "a group",
			args: []string{"task", "--help"},
			want: "usage: app task <subcommand>\n" +
				"\n" +
				"Manage tasks\n" +
				"\n" +
				"Subcommands:\n" +
				"  rm    Delete a task\n" +
				"  list  List the tasks\n" +
				"\n" +
				"Run app task <subcommand> --help for the details of one.\n",
		},
		{
			name: "a leaf with flags",
			args: []string{"task", "rm", "--help"},
			want: "usage: app task rm <task-id> [options]\n" +
				"\n" +
				"Delete a task by its id.\n" +
				"\n" +
				"Options:\n" +
				"  -f, --force        Skip the confirmation\n" +
				"  --ticket <ticket>  Delete only when the task has this ticket, or none when empty\n" +
				"  --title <title>    Delete only when the task has this title\n",
		},
		{
			name: "a leaf without flags",
			args: []string{"task", "list", "--help"},
			want: "usage: app task list\n" +
				"\n" +
				"List the tasks\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls []invocation
			root := newFakeTree(&calls)

			output := captureOutput(func() { _ = root.Execute(testCase.args) })

			if output != testCase.want {
				t.Errorf("expected:\n%s\ngot:\n%s", testCase.want, output)
			}
		})
	}
}

func TestExecute_RefusesBadArgs(t *testing.T) {
	cases := []struct {
		name    string
		args    []string
		wantErr string
	}{
		{
			name:    "a missing positional",
			args:    []string{"task", "rm", "--force"},
			wantErr: "task-id is required, usage: app task rm <task-id> [options]",
		},
		{
			name:    "an extra positional",
			args:    []string{"task", "rm", "42", "43"},
			wantErr: `unexpected argument "43", usage: app task rm <task-id> [options]`,
		},
		{
			name:    "a positional to a leaf taking none",
			args:    []string{"task", "list", "all"},
			wantErr: `unexpected argument "all", usage: app task list`,
		},
		{
			name:    "an unknown flag of a leaf",
			args:    []string{"task", "rm", "42", "-x"},
			wantErr: "flag provided but not defined: -x, usage: app task rm <task-id> [options]",
		},
		{
			name:    "a flag missing its value",
			args:    []string{"task", "rm", "42", "--title"},
			wantErr: "flag needs an argument: -title, usage: app task rm <task-id> [options]",
		},
		{
			name:    "an unknown flag of the root",
			args:    []string{"--quiet", "task"},
			wantErr: "flag provided but not defined: -quiet, usage: app <subcommand>",
		},
		{
			name:    "an unknown subcommand",
			args:    []string{"task", "deploy"},
			wantErr: `unknown subcommand "deploy", usage: app task <subcommand>`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var calls []invocation
			root := newFakeTree(&calls)

			err := root.Execute(testCase.args)

			if err == nil || err.Error() != testCase.wantErr {
				t.Errorf("expected error %q, got %v", testCase.wantErr, err)
			}
			if len(calls) != 0 {
				t.Errorf("expected no run function called, got %+v", calls)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	leaf := func(name string) *Cmd {
		return &Cmd{Name: name, Setup: func(*flag.FlagSet) func(args []string) error {
			return func(args []string) error { return nil }
		}}
	}
	cases := []struct {
		name        string
		root        *Cmd
		shouldPanic bool
	}{
		{
			name:        "a valid tree",
			root:        &Cmd{Name: "app", Subcommands: []*Cmd{leaf("rm"), leaf("list")}},
			shouldPanic: false,
		},
		{
			name:        "a leaf without Setup",
			root:        &Cmd{Name: "app", Subcommands: []*Cmd{{Name: "task", Subcommands: []*Cmd{{Name: "rm"}}}}},
			shouldPanic: true,
		},
		{
			name:        "two subcommands with one name",
			root:        &Cmd{Name: "app", Subcommands: []*Cmd{leaf("rm"), leaf("list"), leaf("rm")}},
			shouldPanic: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			defer func() {
				didPanic := recover() != nil
				if didPanic != testCase.shouldPanic {
					t.Errorf("expected a panic %v, got %v", testCase.shouldPanic, didPanic)
				}
			}()
			testCase.root.Validate()
		})
	}
}

func TestNewRoot_PrintsTheVersion(t *testing.T) {
	root := NewRoot("v1.2.3")

	var err error
	output := captureOutput(func() { err = root.Execute([]string{"--version"}) })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "drg v1.2.3\n" {
		t.Errorf("expected %q, got %q", "drg v1.2.3\n", output)
	}
}

func TestNewRoot_PrintsHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "no arguments", args: nil},
		{name: "the help flag", args: []string{"--help"}},
		{name: "the short help flag", args: []string{"-h"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			root := NewRoot("v1.2.3")
			root.Validate()

			var err error
			output := captureOutput(func() { err = root.Execute(testCase.args) })

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(output, "usage: drg <subcommand>\n") {
				t.Errorf("expected the help to skip the logo outside a terminal, got:\n%s", output)
			}
			lastAt := -1
			for _, sub := range root.Subcommands {
				at := strings.Index(output, "  "+sub.Name+" ")
				if at == -1 {
					t.Fatalf("expected %q in the help, got:\n%s", sub.Name, output)
				}
				if at < lastAt {
					t.Errorf("expected the commands in declaration order, got:\n%s", output)
				}
				lastAt = at
			}
		})
	}
}
