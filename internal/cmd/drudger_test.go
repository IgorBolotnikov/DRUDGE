package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/adapters/persistence"
	"github.com/IgorBolotnikov/DRUDGE/internal/cmd/printer"
	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/config"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

const (
	testProjectName = "Test Project"
	testProjectSlug = "test-project"

	occupiedTaskID = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
)

func TestPrintDrudgers(t *testing.T) {
	cases := []struct {
		name string
		pool []*drudger.Drudger
		// wantLines are substrings the listing prints, in the order given.
		wantLines  []string
		wantAbsent []string
	}{
		{
			name: "idle and occupied Drudgers",
			pool: []*drudger.Drudger{
				{
					Slot:            1,
					Sandbox:         "drudge-claude-test-project-1",
					TaskID:          occupiedTaskID,
					SandboxHealth:   drudger.SandboxUsable,
					WorkspaceHealth: drudger.WorkspaceUsable,
					AgentHealth:     drudger.AgentReady,
					LastChecked:     time.Now().UTC(),
				},
				{
					Slot:    2,
					Sandbox: "drudge-claude-test-project-2",
				},
			},
			wantLines: []string{
				"Drudgers (2):",
				"SLOT",
				"1", "drudge-claude-test-project-1", "a1b2c3d4", "ok", "just now",
				"2", "drudge-claude-test-project-2", "idle", "unchecked", "never",
			},
			// The task id is shortened the way the other listings shorten it.
			wantAbsent: []string{occupiedTaskID},
		},
		{
			name: "a broken sandbox stands out",
			pool: []*drudger.Drudger{
				{
					Slot:            1,
					Sandbox:         "drudge-claude-test-project-1",
					SandboxHealth:   drudger.SandboxGone,
					WorkspaceHealth: drudger.WorkspaceUsable,
					AgentHealth:     drudger.AgentReady,
					LastChecked:     time.Now().UTC(),
				},
				{
					Slot:            2,
					Sandbox:         "drudge-claude-test-project-2",
					SandboxHealth:   drudger.SandboxMisplaced,
					WorkspaceHealth: drudger.WorkspaceUsable,
					AgentHealth:     drudger.AgentReady,
					LastChecked:     time.Now().UTC(),
				},
			},
			wantLines: []string{
				"HEALTH",
				"1", "drudge-claude-test-project-1", "SANDBOX GONE",
				"2", "drudge-claude-test-project-2", "WRONG WORKSPACE",
			},
			wantAbsent: []string{"AGENT REFUSED"},
		},
		{
			name: "a refused agent stands out on a sandbox that is fine",
			pool: []*drudger.Drudger{
				{
					Slot:            1,
					Sandbox:         "drudge-claude-test-project-1",
					SandboxHealth:   drudger.SandboxUsable,
					WorkspaceHealth: drudger.WorkspaceUsable,
					AgentHealth:     drudger.AgentRefused,
					LastChecked:     time.Now().UTC(),
				},
			},
			wantLines: []string{
				"HEALTH",
				"1", "drudge-claude-test-project-1", "AGENT REFUSED",
			},
			// The sandbox is fine, so nothing in the row blames it.
			wantAbsent: []string{"SANDBOX GONE", "WRONG WORKSPACE"},
		},
		{
			name: "a broken workspace stands out on a sandbox that is fine",
			pool: []*drudger.Drudger{
				{
					Slot:            1,
					Sandbox:         "drudge-claude-test-project-1",
					SandboxHealth:   drudger.SandboxUsable,
					WorkspaceHealth: drudger.WorkspaceGone,
					AgentHealth:     drudger.AgentReady,
					LastChecked:     time.Now().UTC(),
				},
				{
					Slot:            2,
					Sandbox:         "drudge-claude-test-project-2",
					SandboxHealth:   drudger.SandboxUsable,
					WorkspaceHealth: drudger.WorkspaceMisplaced,
					AgentHealth:     drudger.AgentReady,
					LastChecked:     time.Now().UTC(),
				},
			},
			wantLines: []string{
				"1", "drudge-claude-test-project-1", "WORKSPACE GONE",
				"2", "drudge-claude-test-project-2", "WRONG WORKTREE",
			},
			wantAbsent: []string{"SANDBOX GONE", "AGENT REFUSED"},
		},
		{
			name: "every broken part is named",
			pool: []*drudger.Drudger{
				{
					Slot:            1,
					Sandbox:         "drudge-claude-test-project-1",
					SandboxHealth:   drudger.SandboxGone,
					WorkspaceHealth: drudger.WorkspaceUsable,
					AgentHealth:     drudger.AgentRefused,
					LastChecked:     time.Now().UTC(),
				},
			},
			wantLines: []string{
				"1", "drudge-claude-test-project-1", "SANDBOX GONE, AGENT REFUSED",
			},
		},
		{
			name: "a sandbox that is fine under an agent nobody has seen work",
			pool: []*drudger.Drudger{
				{
					Slot:            1,
					Sandbox:         "drudge-claude-test-project-1",
					SandboxHealth:   drudger.SandboxUsable,
					WorkspaceHealth: drudger.WorkspaceUsable,
					LastChecked:     time.Now().UTC(),
				},
			},
			wantLines: []string{
				"1", "drudge-claude-test-project-1", "agent unchecked",
			},
			// Only both parts being fine reads as ok.
			wantAbsent: []string{"ok"},
		},
		{
			name:       "no Drudgers yet",
			pool:       nil,
			wantLines:  []string{"· Project " + testProjectSlug + " has no Drudgers"},
			wantAbsent: []string{"SLOT", "DRUDGER", "HEALTH"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			log := common.NewLogger("", common.Labels{})
			output := captureOutput(func() {
				printDrudgers(printer.NewPrinter(log, theme.NewTheme(theme.DefaultTheme())), testProjectSlug, drudgersOnOnePage(testCase.pool), time.Now().UTC())
			})

			rest := output
			for _, want := range testCase.wantLines {
				index := strings.Index(rest, want)
				if index < 0 {
					t.Fatalf("expected the listing to hold %q after the lines before it, got:\n%s", want, output)
				}
				rest = rest[index+len(want):]
			}
			for _, absent := range testCase.wantAbsent {
				if strings.Contains(output, absent) {
					t.Errorf("expected %q to stay out of the listing, got:\n%s", absent, output)
				}
			}
		})
	}
}

func TestPrintDrudgersColors(t *testing.T) {
	pool := []*drudger.Drudger{
		{Slot: 1, Sandbox: testSandboxName(1), SandboxHealth: drudger.SandboxUsable, WorkspaceHealth: drudger.WorkspaceUsable, AgentHealth: drudger.AgentReady},
		{Slot: 2, Sandbox: testSandboxName(2)},
		{Slot: 3, Sandbox: testSandboxName(3), SandboxHealth: drudger.SandboxGone, WorkspaceHealth: drudger.WorkspaceUsable, AgentHealth: drudger.AgentReady},
		{Slot: 4, Sandbox: testSandboxName(4), WorkspaceHealth: drudger.WorkspaceUsable, AgentHealth: drudger.AgentRefused},
		{Slot: 5, Sandbox: testSandboxName(5), SandboxHealth: "hand-edited", WorkspaceHealth: drudger.WorkspaceUsable, AgentHealth: drudger.AgentReady},
	}
	// plainHealths are the health cells of the Drudgers above, without color.
	plainHealths := []string{"ok", "unchecked", "SANDBOX GONE", "sandbox unchecked, AGENT REFUSED", "hand-edited"}

	header := fmt.Sprintf("Drudgers (%d):\n", len(pool)) +
		fmt.Sprintf("  SLOT  %-40s  TASK      %-*s  LAST CHECKED\n", "DRUDGER", healthColumnWidth, "HEALTH") +
		fmt.Sprintf("  ----  %s  --------  %s  ------------\n", strings.Repeat("-", 40), strings.Repeat("-", healthColumnWidth))
	rowLine := func(slot int, healthCell string) string {
		return fmt.Sprintf("  %-4d  %-40s  %-8s  %s  never\n", slot, testSandboxName(slot), idleLabel, healthCell)
	}
	plainRows := func() string {
		var lines strings.Builder
		for index, health := range plainHealths {
			lines.WriteString(rowLine(index+1, fmt.Sprintf("%-*s", healthColumnWidth, health)))
		}
		return lines.String()
	}

	// healthRoles repeats the roles of the health labels on purpose. A test
	// that reads them from the production map passes whatever roles it holds.
	healthRoles := map[string]string{
		healthOkLabel:           theme.RoleSuccess,
		healthUncheckedLabel:    theme.RoleMuted,
		sandboxUncheckedLabel:   theme.RoleMuted,
		workspaceUncheckedLabel: theme.RoleMuted,
		agentUncheckedLabel:     theme.RoleMuted,
		sandboxGoneLabel:        theme.RoleError,
		sandboxMisplacedLabel:   theme.RoleError,
		workspaceGoneLabel:      theme.RoleError,
		workspaceMisplacedLabel: theme.RoleError,
		agentRefusedLabel:       theme.RoleError,
	}
	coloredRows := func(palette *theme.Theme) string {
		var lines strings.Builder
		for index, health := range plainHealths {
			labels := strings.Split(health, healthPartSeparator)
			for labelIndex, label := range labels {
				role, ok := healthRoles[label]
				if !ok {
					continue
				}
				labels[labelIndex] = palette.Paint(theme.Stdout, role, label)
			}
			padding := strings.Repeat(" ", healthColumnWidth-len(health))
			lines.WriteString(rowLine(index+1, strings.Join(labels, healthPartSeparator)+padding))
		}
		return lines.String()
	}

	cases := []struct {
		name string
		env  map[string]string
		want func(palette *theme.Theme) string
	}{
		{
			name: "forced color paints each label in its role, leaving the separator plain",
			env:  map[string]string{"FORCE_COLOR": "1"},
			want: func(palette *theme.Theme) string { return header + coloredRows(palette) },
		},
		{
			name: "no color prints every label plain",
			env:  map[string]string{"NO_COLOR": "1", "FORCE_COLOR": "1"},
			want: func(*theme.Theme) string { return header + plainRows() },
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "")
			for name, value := range testCase.env {
				t.Setenv(name, value)
			}
			log := common.NewLogger("", common.Labels{})

			var out string
			errOut := captureStderr(func() {
				out = captureOutput(func() {
					printDrudgers(printer.NewPrinter(log, theme.NewTheme(theme.DefaultTheme())), testProjectSlug, drudgersOnOnePage(pool), time.Time{})
				})
			})

			if want := testCase.want(theme.NewTheme(theme.DefaultTheme())); out != want {
				t.Errorf("expected:\n%q\ngot:\n%q", want, out)
			}
			if errOut != "" {
				t.Errorf("expected nothing on stderr, got %q", errOut)
			}
		})
	}
}

func drudgersOnOnePage(pool []*drudger.Drudger) common.Page[*drudger.Drudger] {
	return common.Page[*drudger.Drudger]{Items: pool, Number: 1, TotalPages: 1, TotalItems: len(pool)}
}

func TestPrintReclaimed(t *testing.T) {
	cases := []struct {
		name  string
		freed []drudger.FreedSlot
		want  string
	}{
		{
			name: "freed slots",
			freed: []drudger.FreedSlot{
				{Slot: 1, Sandbox: testSandboxName(1), TaskID: occupiedTaskID, Reason: "its sandbox is not running"},
				{Slot: 3, Sandbox: testSandboxName(3), TaskID: "3f9a1c2e-0b1d-4c2e-9f3a-1c2e0b1d4c2e", Reason: "its launch never made a run directory"},
			},
			want: "✓ Freed Drudger 1 (" + testSandboxName(1) + "), it held task a1b2c3d4 with no agent in it, its sandbox is not running\n" +
				"✓ Freed Drudger 3 (" + testSandboxName(3) + "), it held task 3f9a1c2e with no agent in it, its launch never made a run directory\n" +
				"    Next  start a task over with drg task rerun <task-id>\n",
		},
		{
			name: "nothing to reclaim",
			want: "· Every Drudger of project " + testProjectSlug + " is either idle or working, nothing to reclaim\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")
			palette := theme.NewTheme(theme.DefaultTheme())
			output := captureOutput(func() {
				printReclaimed(printer.NewPrinter(newThemedLogger(palette), palette), testProjectSlug, testCase.freed)
			})
			if output != testCase.want {
				t.Errorf("expected:\n%q\ngot:\n%q", testCase.want, output)
			}
		})
	}
}

func TestDrudgerList(t *testing.T) {
	header := fmt.Sprintf("  SLOT  %-40s  TASK      %-*s  LAST CHECKED\n", "DRUDGER", healthColumnWidth, "HEALTH") +
		fmt.Sprintf("  ----  %s  --------  %s  ------------\n", strings.Repeat("-", 40), strings.Repeat("-", healthColumnWidth))
	row := func(slot int) string {
		return fmt.Sprintf("  %-4d  %-40s  %-8s  %-*s  never\n", slot, testSandboxName(slot), idleLabel, healthColumnWidth, healthUncheckedLabel)
	}

	cases := []struct {
		name  string
		slots []int
		args  []string
		// localDrudger is the drudger section of the local config when set.
		localDrudger string
		want         string
		wantErr      string
	}{
		{
			name:  "the first of two pages hints at the next one",
			slots: []int{3, 1, 2},
			args:  []string{"--page-size", "2"},
			want:  "Drudgers (3):\n" + header + row(1) + row(2) + "Page 1 of 2, see the next one with --page 2\n",
		},
		{
			name:  "the last of two pages prints a footer without a hint",
			slots: []int{3, 1, 2},
			args:  []string{"-p", "2", "--page-size", "2"},
			want:  "Drudgers (3):\n" + header + row(3) + "Page 2 of 2\n",
		},
		{
			name:         "the local config sets the page size",
			slots:        []int{1, 2},
			args:         []string{"--page", "2"},
			localDrudger: `{"pageSize": 1}`,
			want:         "Drudgers (2):\n" + header + row(2) + "Page 2 of 2\n",
		},
		{
			name:         "a page size of 0 on the command line turns paging off",
			slots:        []int{1, 2},
			args:         []string{"--page-size", "0"},
			localDrudger: `{"pageSize": 1}`,
			want:         "Drudgers (2):\n" + header + row(1) + row(2),
		},
		{
			name: "no Drudgers",
			want: "· Project " + testProjectSlug + " has no Drudgers, the first one is built when you run a task\n",
		},
		{
			name:    "a page past the last one",
			slots:   []int{1, 2, 3},
			args:    []string{"--page", "3", "--page-size", "2"},
			wantErr: "page 3 does not exist, there are 2 pages of Drudgers",
		},
		{
			name:    "a negative page size",
			args:    []string{"--page-size", "-1"},
			wantErr: "--page-size must be 0 or more, got -1",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			t.Setenv("NO_COLOR", "1")
			t.Chdir(t.TempDir())
			localConfig := &config.LocalConfig{ProjectSlug: testProjectSlug}
			if testCase.localDrudger != "" {
				if err := json.Unmarshal([]byte(testCase.localDrudger), &localConfig.Drudger); err != nil {
					t.Fatal(err)
				}
			}
			if err := localConfig.Save(); err != nil {
				t.Fatal(err)
			}
			pool := make([]*drudger.Drudger, 0, len(testCase.slots))
			for _, slot := range testCase.slots {
				pool = append(pool, &drudger.Drudger{Slot: slot, Sandbox: testSandboxName(slot)})
			}
			err := persistence.NewFileDrudgerRepository("").UpdateDrudgers(testProjectSlug, func([]*drudger.Drudger) ([]*drudger.Drudger, error) {
				return pool, nil
			})
			if err != nil {
				t.Fatal(err)
			}

			args := append([]string{DrudgerCmd.Name, "list"}, testCase.args...)
			output := captureOutput(func() { err = NewRoot("v1.2.3").Execute(args) })

			if testCase.wantErr != "" {
				if err == nil || err.Error() != testCase.wantErr {
					t.Fatalf("error = %v, want %q", err, testCase.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if output != testCase.want {
				t.Errorf("expected:\n%q\ngot:\n%q", testCase.want, output)
			}
		})
	}
}

func testSandboxName(slot int) string {
	return fmt.Sprintf("drudge-claude-%s-%d", testProjectSlug, slot)
}

func TestDrudgerList_NoProjectInDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	err := drudgerList(pageFlags{number: 1})
	if err == nil {
		t.Fatal("expected an error in a directory linked to no project")
	}
	if !strings.Contains(err.Error(), "no project initialized") {
		t.Errorf("expected the error to say no project is initialized, got %q", err)
	}
}

func TestRunDrudger_PrintsHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "the help flag", args: []string{DrudgerCmd.Name, "--help"}},
		{name: "no subcommand", args: []string{DrudgerCmd.Name}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			var err error
			output := captureOutput(func() { err = NewRoot("v1.2.3").Execute(testCase.args) })

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.HasPrefix(output, "usage: drg drudger <subcommand>\n") {
				t.Errorf("expected the help of the drudger command, got:\n%s", output)
			}
			for _, name := range []string{"list", "nuke", "reclaim"} {
				if !strings.Contains(output, "  "+name+" ") {
					t.Errorf("expected %q in the help, got:\n%s", name, output)
				}
			}
		})
	}
}

func TestRunDrudger_UnknownSubcommand(t *testing.T) {
	err := NewRoot("v1.2.3").Execute([]string{DrudgerCmd.Name, "frobnicate"})

	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), `unknown subcommand "frobnicate"`) {
		t.Errorf("expected the error to name the unknown subcommand, got %q", err)
	}
}

func TestParseDrudgerNukeArgs(t *testing.T) {
	cases := []struct {
		name     string
		slot     string
		wantSlot int
		wantErr  bool
	}{
		{name: "a slot", slot: "2", wantSlot: 2},
		{name: "a slot that is not a number", slot: "one", wantErr: true},
		{name: "a slot that is not whole", slot: "1.5", wantErr: true},
		{name: "slot zero", slot: "0", wantErr: true},
		{name: "a negative slot", slot: "-1", wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			slot, err := parseDrudgerNukeArgs(testCase.slot)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error for slot %q", testCase.slot)
				}
				if !strings.Contains(err.Error(), "slots are whole numbers starting at 1") {
					t.Errorf("expected the error to say what a slot is, got %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if slot != testCase.wantSlot {
				t.Errorf("expected slot %d, got %d", testCase.wantSlot, slot)
			}
		})
	}
}

func TestFormatHealth(t *testing.T) {
	cases := []struct {
		name      string
		sandbox   drudger.SandboxHealth
		workspace drudger.WorkspaceHealth
		agent     drudger.AgentHealth
		want      string
	}{
		{name: "never looked at any part", want: "unchecked"},
		{name: "every part is fine", sandbox: drudger.SandboxUsable, workspace: drudger.WorkspaceUsable, agent: drudger.AgentReady, want: "ok"},
		{name: "the sandbox is not there", sandbox: drudger.SandboxGone, workspace: drudger.WorkspaceUsable, agent: drudger.AgentReady, want: "SANDBOX GONE"},
		{name: "the sandbox holds another repository", sandbox: drudger.SandboxMisplaced, workspace: drudger.WorkspaceUsable, agent: drudger.AgentReady, want: "WRONG WORKSPACE"},
		{name: "the worktrees are gone", sandbox: drudger.SandboxUsable, workspace: drudger.WorkspaceGone, agent: drudger.AgentReady, want: "WORKSPACE GONE"},
		{name: "the workspace path holds something else", sandbox: drudger.SandboxUsable, workspace: drudger.WorkspaceMisplaced, agent: drudger.AgentReady, want: "WRONG WORKTREE"},
		{name: "the vendor turned the agent away", sandbox: drudger.SandboxUsable, workspace: drudger.WorkspaceUsable, agent: drudger.AgentRefused, want: "AGENT REFUSED"},
		{name: "every part is broken", sandbox: drudger.SandboxGone, workspace: drudger.WorkspaceGone, agent: drudger.AgentRefused, want: "SANDBOX GONE, WORKSPACE GONE, AGENT REFUSED"},
		{name: "the agent has not been seen work yet", sandbox: drudger.SandboxUsable, workspace: drudger.WorkspaceUsable, want: "agent unchecked"},
		{name: "the sandbox has not been looked at yet", workspace: drudger.WorkspaceUsable, agent: drudger.AgentReady, want: "sandbox unchecked"},
		{name: "the workspace has not been looked at yet", sandbox: drudger.SandboxUsable, agent: drudger.AgentReady, want: "workspace unchecked"},
		{name: "a sandbox value this build does not know", sandbox: "hand-edited", workspace: drudger.WorkspaceUsable, agent: drudger.AgentReady, want: "hand-edited"},
		{name: "a workspace value this build does not know", sandbox: drudger.SandboxUsable, workspace: "hand-edited", agent: drudger.AgentReady, want: "hand-edited"},
		{name: "an agent value this build does not know", sandbox: drudger.SandboxUsable, workspace: drudger.WorkspaceUsable, agent: "hand-edited", want: "hand-edited"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			entry := &drudger.Drudger{SandboxHealth: testCase.sandbox, WorkspaceHealth: testCase.workspace, AgentHealth: testCase.agent}

			got := formatHealth(entry)
			if got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestFormatAgo(t *testing.T) {
	now := time.Date(2026, 9, 6, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name        string
		lastChecked time.Time
		want        string
	}{
		{name: "never looked", lastChecked: time.Time{}, want: "never"},
		{name: "seconds ago", lastChecked: now.Add(-30 * time.Second), want: "just now"},
		{name: "minutes ago", lastChecked: now.Add(-5 * time.Minute), want: "5m ago"},
		{name: "hours ago", lastChecked: now.Add(-3 * time.Hour), want: "3h ago"},
		{name: "days ago", lastChecked: now.Add(-50 * time.Hour), want: "2d ago"},
		{name: "stamped in the future", lastChecked: now.Add(time.Minute), want: "just now"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := formatAgo(testCase.lastChecked, now)
			if got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func captureOutput(f func()) string {
	orig := os.Stdout
	reader, writer, _ := os.Pipe()
	os.Stdout = writer
	f()
	writer.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(reader)
	return string(out)
}
