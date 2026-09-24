package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
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
			wantLines:  []string{"has no Drudgers"},
			wantAbsent: []string{"SLOT", "DRUDGER", "HEALTH"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			log := common.NewLogger("")
			output := captureOutput(func() { printDrudgers(log, testProjectSlug, testCase.pool, time.Now().UTC()) })

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

func TestDrudgerList_NoProjectInDirectory(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Chdir(t.TempDir())

	err := drudgerList(nil)
	if err == nil {
		t.Fatal("expected an error in a directory linked to no project")
	}
	if !strings.Contains(err.Error(), "no project initialized") {
		t.Errorf("expected the error to say no project is initialized, got %q", err)
	}
}

func TestDrudgerReclaim_BadArgs(t *testing.T) {
	err := drudgerReclaim([]string{"1"})
	if err == nil {
		t.Fatal("expected an error for an argument the subcommand does not take")
	}
	if !strings.Contains(err.Error(), drudgerReclaimUsage) {
		t.Errorf("expected the error to hold the usage, got %q", err)
	}
}

func TestRunDrudger_BadArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "no subcommand", args: nil},
		{name: "unknown subcommand", args: []string{"frobnicate"}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if err := runDrudger(testCase.args); err == nil {
				t.Fatalf("expected an error for args %v", testCase.args)
			}
		})
	}
}

func TestParseDrudgerNukeArgs(t *testing.T) {
	cases := []struct {
		name      string
		args      []string
		wantSlot  int
		wantForce bool
		wantErr   bool
	}{
		{name: "a slot alone", args: []string{"2"}, wantSlot: 2},
		{name: "a slot and the short force flag", args: []string{"2", "-f"}, wantSlot: 2, wantForce: true},
		{name: "a slot and the long force flag", args: []string{"2", "--force"}, wantSlot: 2, wantForce: true},
		{name: "the force flag first", args: []string{"-f", "2"}, wantSlot: 2, wantForce: true},
		{name: "no slot", args: nil, wantErr: true},
		{name: "the force flag alone", args: []string{"-f"}, wantErr: true},
		{name: "two slots", args: []string{"1", "2"}, wantErr: true},
		{name: "an unknown flag", args: []string{"1", "--yolo"}, wantErr: true},
		{name: "a slot that is not a number", args: []string{"one"}, wantErr: true},
		{name: "slot zero", args: []string{"0"}, wantErr: true},
		{name: "a negative slot reads as a flag", args: []string{"-1"}, wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			slot, isForced, err := parseDrudgerNukeArgs(testCase.args)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error for args %v", testCase.args)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if slot != testCase.wantSlot {
				t.Errorf("expected slot %d, got %d", testCase.wantSlot, slot)
			}
			if isForced != testCase.wantForce {
				t.Errorf("expected force %v, got %v", testCase.wantForce, isForced)
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
