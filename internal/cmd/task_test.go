package cmd

import (
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/drudger"
	"drudge/internal/task"
)

func TestParseTaskRunArgs(t *testing.T) {
	cases := []struct {
		name       string
		subcommand string
		usage      string
		args       []string
		wantTaskID task.TaskID
		wantDryRun bool
		wantErr    bool
		// wantErrText is a fragment the error must carry, checked when set.
		wantErrText string
	}{
		{
			name:       "task ID only",
			args:       []string{"abc123"},
			wantTaskID: "abc123",
		},
		{
			name:       "task ID with dry run flag",
			args:       []string{"abc123", "--dry-run"},
			wantTaskID: "abc123",
			wantDryRun: true,
		},
		{
			name:       "dry run flag before the task ID",
			args:       []string{"--dry-run", "abc123"},
			wantTaskID: "abc123",
			wantDryRun: true,
		},
		{
			name:    "no arguments",
			args:    nil,
			wantErr: true,
		},
		{
			name:    "dry run flag without a task ID",
			args:    []string{"--dry-run"},
			wantErr: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"abc123", "--detached"},
			wantErr: true,
		},
		{
			name:    "second task ID",
			args:    []string{"abc123", "def456"},
			wantErr: true,
		},
		{
			name:        "the error names the subcommand the user typed",
			subcommand:  rerunSubcommand,
			usage:       taskRerunUsage,
			args:        []string{"abc123", "def456"},
			wantErr:     true,
			wantErrText: "drg task rerun",
		},
		{
			name:       "rerun takes the same arguments",
			subcommand: rerunSubcommand,
			usage:      taskRerunUsage,
			args:       []string{"abc123", "--dry-run"},
			wantTaskID: "abc123",
			wantDryRun: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			subcommand := testCase.subcommand
			if subcommand == "" {
				subcommand = runSubcommand
			}
			usage := testCase.usage
			if usage == "" {
				usage = taskRunUsage
			}

			taskID, isDryRun, err := parseTaskRunArgs(testCase.args, subcommand, usage)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got task ID %q and dry run %v", taskID, isDryRun)
				}
				if testCase.wantErrText != "" && !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Errorf("expected the error to name %q, got %q", testCase.wantErrText, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if taskID != testCase.wantTaskID {
				t.Errorf("expected task ID %q, got %q", testCase.wantTaskID, taskID)
			}
			if isDryRun != testCase.wantDryRun {
				t.Errorf("expected dry run %v, got %v", testCase.wantDryRun, isDryRun)
			}
		})
	}
}

func TestParseTaskIDArgs(t *testing.T) {
	cases := []struct {
		name       string
		subcommand string
		usage      string
		args       []string
		wantTaskID task.TaskID
		wantErr    bool
		// wantErrText is a fragment the error must carry, checked when set.
		wantErrText string
	}{
		{
			name:       "task ID only",
			args:       []string{"abc123"},
			wantTaskID: "abc123",
		},
		{
			name:       "an id prefix is passed on as typed",
			args:       []string{"00"},
			wantTaskID: "00",
		},
		{
			name:    "no arguments",
			args:    nil,
			wantErr: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"abc123", "--watch"},
			wantErr: true,
		},
		{
			name:    "second task ID",
			args:    []string{"abc123", "def456"},
			wantErr: true,
		},
		{
			name:        "the error names the subcommand the user typed",
			subcommand:  showSubcommand,
			usage:       taskShowUsage,
			args:        []string{"abc123", "def456"},
			wantErr:     true,
			wantErrText: "drg task show",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			subcommand := testCase.subcommand
			if subcommand == "" {
				subcommand = statusSubcommand
			}
			usage := testCase.usage
			if usage == "" {
				usage = taskStatusUsage
			}

			taskID, err := parseTaskIDArgs(testCase.args, subcommand, usage)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got task ID %q", taskID)
				}
				if testCase.wantErrText != "" && !strings.Contains(err.Error(), testCase.wantErrText) {
					t.Errorf("expected the error to name %q, got %q", testCase.wantErrText, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if taskID != testCase.wantTaskID {
				t.Errorf("expected task ID %q, got %q", testCase.wantTaskID, taskID)
			}
		})
	}
}

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
		name      string
		args      []string
		wantUsage string
		wantTexts []string
	}{
		{name: "the task command with no subcommand", args: nil, wantUsage: taskUsage},
		{name: "the task command", args: []string{helpFlag}, wantUsage: taskUsage},
		{name: "the task command with the short flag", args: []string{helpFlagShort}, wantUsage: taskUsage},
		{
			name:      "new",
			args:      []string{newSubcommand, helpFlag},
			wantUsage: taskNewUsage,
			wantTexts: []string{titleFlag, descriptionFlag, descriptionFileFlag, ticketFlag, statusFlag, blockedByFlag, parentFlag},
		},
		{name: "new with the short flag", args: []string{newSubcommand, helpFlagShort}, wantUsage: taskNewUsage},
		{name: "new with a title before the flag", args: []string{newSubcommand, titleFlag, "Wire the service", helpFlag}, wantUsage: taskNewUsage},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			home := t.TempDir()
			t.Setenv("HOME", home)
			t.Chdir(t.TempDir())

			var err error
			output := captureOutput(func() { err = runTask(testCase.args) })

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(output, testCase.wantUsage+"\n") {
				t.Errorf("expected the help to start with %q, got:\n%s", testCase.wantUsage, output)
			}
			for _, want := range testCase.wantTexts {
				if !strings.Contains(output, want) {
					t.Errorf("expected the help to mention %q, got:\n%s", want, output)
				}
			}
			entries, err := os.ReadDir(home)
			if err != nil {
				t.Fatalf("cannot read the home directory: %v", err)
			}
			if len(entries) != 0 {
				t.Errorf("expected the help to write nothing, found %d entries in the home directory", len(entries))
			}
		})
	}
}

func TestRunTask_UnknownSubcommand(t *testing.T) {
	if err := runTask([]string{"frobnicate"}); err == nil {
		t.Fatal("expected an error for an unknown task subcommand")
	}
}

func TestTaskNew_RefusesEditOnlyBlockerFlags(t *testing.T) {
	for _, flag := range []string{blockFlag, unblockFlag} {
		t.Run(flag, func(t *testing.T) {
			err := taskNew([]string{"--title", "Wire the service", "--description", "", flag, "9c8d"})
			if err == nil {
				t.Fatalf("expected drg task new to refuse %s", flag)
			}
			if !strings.Contains(err.Error(), flag) || !strings.Contains(err.Error(), blockedByFlag) {
				t.Errorf("expected the error to name %s and %s, got %q", flag, blockedByFlag, err)
			}
		})
	}
}

func TestParseTaskNewArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		// files are written to the directory the parsing runs in, by name.
		files   map[string]string
		stdin   string
		wantDto task.CreateTaskDto
		wantErr bool
		// wantErrText is a fragment the error must carry, checked when set.
		wantErrText string
	}{
		{
			name:    "a title and a description",
			args:    []string{"--title", "Fix logout", "--description", "SSO logs nobody out"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: "SSO logs nobody out", Status: task.StatusDraft, BlockedBy: []task.TaskID{}},
		},
		{
			name:    "an empty description",
			args:    []string{"--title", "Fix logout", "--description", ""},
			wantDto: task.CreateTaskDto{Title: "Fix logout", Status: task.StatusDraft, BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a known status",
			args:    []string{"--title", "Fix logout", "--description", "", "--status", "todo"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", Status: task.StatusTodo, BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a ticket",
			args:    []string{"--title", "Fix logout", "--description", "", "--ticket", "R-004-01"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", Status: task.StatusDraft, TicketID: "R-004-01", BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a list of blockers",
			args:    []string{"--title", "Fix logout", "--description", "", "--blocked-by", "9c8d7e6f, 1a2b3c4d"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", Status: task.StatusDraft, BlockedBy: []task.TaskID{"9c8d7e6f", "1a2b3c4d"}},
		},
		{
			name:    "a parent",
			args:    []string{"--title", "Fix logout", "--description", "", "--parent", "9c8d7e6f"},
			wantDto: task.CreateTaskDto{Title: "Fix logout", Status: task.StatusDraft, BlockedBy: []task.TaskID{}, ParentTaskID: "9c8d7e6f"},
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
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: "SSO logs nobody out", Status: task.StatusDraft, BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a description from stdin",
			args:    []string{"--title", "Fix logout", "--description-file", "-"},
			stdin:   "SSO logs nobody out",
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: "SSO logs nobody out", Status: task.StatusDraft, BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a description ending in a newline",
			args:    []string{"--title", "Fix logout", "--description-file", "-"},
			stdin:   "SSO logs nobody out\n\n",
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: "SSO logs nobody out\n", Status: task.StatusDraft, BlockedBy: []task.TaskID{}},
		},
		{
			name:    "a description with a code block and dollar signs",
			args:    []string{"--title", "Fix logout", "--description-file", "-"},
			stdin:   literalDescription + "\n",
			wantDto: task.CreateTaskDto{Title: "Fix logout", Description: literalDescription, Status: task.StatusDraft, BlockedBy: []task.TaskID{}},
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
			name:        "a description file flag with no path",
			args:        []string{"--title", "Fix logout", "--description-file"},
			wantErr:     true,
			wantErrText: "--description-file needs a path",
		},
		{
			name:        "an unknown status",
			args:        []string{"--title", "Fix logout", "--description", "", "--status", "someday"},
			wantErr:     true,
			wantErrText: `invalid status "someday"`,
		},
		{
			name:        "blockers to add",
			args:        []string{"--title", "Fix logout", "--description", "", "--block", "9c8d"},
			wantErr:     true,
			wantErrText: "drg task new takes no --block, name the blockers of a new task with --blocked-by",
		},
		{
			name:        "blockers to remove",
			args:        []string{"--title", "Fix logout", "--description", "", "--unblock", "9c8d"},
			wantErr:     true,
			wantErrText: "drg task new takes no --unblock, name the blockers of a new task with --blocked-by",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			writeFiles(t, testCase.files)

			dto, err := parseTaskNewArgs(testCase.args, strings.NewReader(testCase.stdin))

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
