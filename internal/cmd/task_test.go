package cmd

import (
	"flag"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestPrintSessionStatus(t *testing.T) {
	cases := []struct {
		name   string
		report drudger.SessionReport
		want   []string
	}{
		{
			name: "an agent still working",
			report: drudger.SessionReport{
				Status:    drudger.StatusWorking,
				RunDir:    "/tmp/run",
				SessionID: "ebe60e03",
				LastWrite: time.Now(),
			},
			want: []string{"working", "ebe60e03", "/tmp/run", "just now"},
		},
		{
			name: "an agent that has not reported its session yet",
			report: drudger.SessionReport{
				Status:    drudger.StatusNeedsBabysitting,
				LastWrite: time.Now().Add(-2 * time.Hour),
			},
			want: []string{"needs babysitting", notReportedLabel, "2h ago"},
		},
		{
			name: "a finished run",
			report: drudger.SessionReport{
				Status:    drudger.StatusGotShitDone,
				LastWrite: time.Now(),
				ExitCode:  0,
				Result: &drudger.SessionResult{
					Subtype:  "success",
					NumTurns: 3,
					Duration: 8664 * time.Millisecond,
					CostUSD:  0.0695,
					Text:     "Fixed the login",
				},
			},
			want: []string{"got shit done", "3", "9s", "$0.0695", "Fixed the login"},
		},
		{
			name: "a run that fell over",
			report: drudger.SessionReport{
				Status:    drudger.StatusFuckedUp,
				LastWrite: time.Now(),
				ExitCode:  137,
			},
			want: []string{"fucked up", "137"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			session := &drudger.TaskSession{
				Task:   &task.Task{ID: "abc123", Title: "Fix login"},
				Report: testCase.report,
			}
			log := common.NewLogger("")

			out := captureOutput(func() { printSessionStatus(log, session) })

			for _, want := range append(testCase.want, "abc123", "Fix login") {
				if !strings.Contains(out, want) {
					t.Errorf("expected the report to mention %q, got:\n%s", want, out)
				}
			}
		})
	}
}

func TestRunTask_PrintsHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "the help flag", args: []string{TaskCmd.Name, helpFlag}},
		{name: "no subcommand", args: []string{TaskCmd.Name}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var err error
			output := captureOutput(func() { err = NewRoot("v1.2.3").Execute(testCase.args) })

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(output, "usage: drg task <subcommand>\n") {
				t.Errorf("expected the help of the task command, got:\n%s", output)
			}
			for _, name := range []string{"new", "list", "next", "show", "edit", "rm", "run", "rerun", "status"} {
				if !strings.Contains(output, "  "+name+" ") {
					t.Errorf("expected %q in the help, got:\n%s", name, output)
				}
			}
		})
	}
}

func TestRunTask_UnknownSubcommand(t *testing.T) {
	err := NewRoot("v1.2.3").Execute([]string{TaskCmd.Name, "frobnicate"})

	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), `unknown subcommand "frobnicate"`) {
		t.Errorf("expected the error to name the unknown subcommand, got %q", err)
	}
}

func TestTaskNew_RefusesEditOnlyBlockerFlags(t *testing.T) {
	for _, name := range []string{blockFlagName, unblockFlagName} {
		t.Run(name, func(t *testing.T) {
			err := NewRoot("v1.2.3").Execute([]string{TaskCmd.Name, "new", "--title", "Wire the service", "--description", "", flagLabel(name), "9c8d"})

			if err == nil {
				t.Fatalf("expected drg task new to refuse %s", flagLabel(name))
			}
			if !strings.Contains(err.Error(), "flag provided but not defined: -"+name) {
				t.Errorf("expected the error to name the undefined flag %s, got %q", name, err)
			}
		})
	}
}

func TestTaskNewFlags(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// files are written to the directory the flags are read in, by name.
		files   map[string]string
		stdin   string
		wantDto task.CreateTaskDto
		wantErr bool
		// wantErrText is a fragment the error must carry, checked when set.
		wantErrText string
	}{
		{
			name:    "a title and a description with no status",
			args:    []string{"--title", "Fix logout", "--description", "SSO logs nobody out"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: "SSO logs nobody out", BlockedBy: []task.TaskID{}},
		},
		{
			name:    "an empty description",
			args:    []string{"--title", "Fix logout", "--description", ""},
			wantDto: task.CreateTaskDto{Title: "Fix logout", BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a known status",
			args:    []string{"--title", "Fix logout", "--description", "", "--status", "todo"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", Status: task.StatusTodo, BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a ticket",
			args:    []string{"--title", "Fix logout", "--description", "", "--ticket", "R-004-01"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", TicketID: "R-004-01", BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a list of blockers",
			args:    []string{"--title", "Fix logout", "--description", "", "--blocked-by", "9c8d7e6f, 1a2b3c4d"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", BlockedBy: []task.TaskID{"9c8d7e6f", "1a2b3c4d"}},
		},
		{
			name:    "a parent",
			args:    []string{"--title", "Fix logout", "--description", "", "--parent", "9c8d7e6f"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", BlockedBy: []task.TaskID{}, ParentTaskID: "9c8d7e6f"},
		},
		{
			name:        "no title",
			args:        []string{"--description", "SSO logs nobody out"},
			wantErr:     true,
			wantErrText: "--title is required",
		},
		{
			name:        "an empty title",
			args:        []string{"--title", "", "--description", "SSO logs nobody out"},
			wantErr:     true,
			wantErrText: "--title is required",
		},
		{
			name:        "no description",
			args:        []string{"--title", "Fix logout"},
			wantErr:     true,
			wantErrText: "--description or --description-file is required",
		},
		{
			name:    "a description file",
			args:    []string{"--title", "Fix logout", "--description-file", "description.md"},
			files:   map[string]string{"description.md": "SSO logs nobody out"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: "SSO logs nobody out", BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a description from stdin",
			args:    []string{"--title", "Fix logout", "--description-file", "-"},
			stdin:   "SSO logs nobody out",
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: "SSO logs nobody out", BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a description ending in a newline",
			args:    []string{"--title", "Fix logout", "--description-file", "-"},
			stdin:   "SSO logs nobody out\n\n",
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: "SSO logs nobody out\n", BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a description with a code block and dollar signs",
			args:    []string{"--title", "Fix logout", "--description-file", "-"},
			stdin:   literalDescription + "\n",
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: literalDescription, BlockedBy: []task.TaskID{}},
		},
		{
			name:        "a description and a description file",
			args:        []string{"--title", "Fix logout", "--description", "SSO logs nobody out", "--description-file", "-"},
			stdin:       "SSO logs nobody out",
			wantErr:     true,
			wantErrText: "--description and --description-file cannot be used together",
		},
		{
			name:        "a missing description file",
			args:        []string{"--title", "Fix logout", "--description-file", "missing.md"},
			wantErr:     true,
			wantErrText: `"missing.md"`,
		},
		{
			name:        "an empty description file",
			args:        []string{"--title", "Fix logout", "--description-file", "description.md"},
			files:       map[string]string{"description.md": ""},
			wantErr:     true,
			wantErrText: `"description.md" holds no description`,
		},
		{
			name:        "a whitespace-only description from stdin",
			args:        []string{"--title", "Fix logout", "--description-file", "-"},
			stdin:       " \n\t\n",
			wantErr:     true,
			wantErrText: "stdin holds no description",
		},
		{
			name:        "an unknown status",
			args:        []string{"--title", "Fix logout", "--description", "", "--status", "someday"},
			wantErr:     true,
			wantErrText: `invalid status "someday"`,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			writeFiles(t, testCase.files)

			flags := &taskNewFlags{}
			parseTestFlags(t, flags.declare, testCase.args)

			dto, err := flags.dto(strings.NewReader(testCase.stdin))

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", dto)
				}
				if testCase.wantErrText != "" && !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Errorf("expected the error to name %q, got %q", testCase.wantErrText, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(dto, testCase.wantDto) {
				t.Errorf("expected %+v, got %+v", testCase.wantDto, dto)
			}
		})
	}
}

// literalDescription holds what a shell would expand or a heredoc might break.
const literalDescription = "Run `make drg task list` and check $HOME.\n\n```sh\necho \"$1\" '$2' \\$3\n```"

// parseTestFlags declares flags into a fresh flag set and parses args with it.
func parseTestFlags(t *testing.T, declare func(fs *flag.FlagSet), args []string) {
	t.Helper()
	fs := flag.NewFlagSet(t.Name(), flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	declare(fs)
	if err := fs.Parse(args); err != nil {
		t.Fatalf("parsing %q: %v", args, err)
	}
}

// writeFiles runs the test in a fresh directory holding the given files.
func writeFiles(t *testing.T, files map[string]string) {
	t.Helper()
	t.Chdir(t.TempDir())
	for name, content := range files {
		if err := os.WriteFile(name, []byte(content), 0o644); err != nil {
			t.Fatalf("writing %s: %v", name, err)
		}
	}
}
