package cmd

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"drudge/internal/adapters/persistence"
	"drudge/internal/drudger"
)

const (
	testProjectName = "Test Project"
	testProjectSlug = "test-project"

	occupiedTaskID = "a1b2c3d4-e5f6-7890-abcd-ef1234567890"
)

func TestDrudgerList(t *testing.T) {
	cases := []struct {
		name string
		pool []*drudger.Drudger
		// wantLines are substrings the listing prints, in the order given.
		wantLines  []string
		wantAbsent []string
	}{
		{
			name: "idle and occupied Drudgers, lowest slot first",
			pool: []*drudger.Drudger{
				{
					Slot:    2,
					Sandbox: "drudge-claude-test-project-2",
				},
				{
					Slot:        1,
					Sandbox:     "drudge-claude-test-project-1",
					TaskID:      occupiedTaskID,
					LastChecked: time.Now().UTC(),
				},
			},
			wantLines: []string{
				"Drudgers (2):",
				"SLOT",
				"1", "drudge-claude-test-project-1", "a1b2c3d4", "just now",
				"2", "drudge-claude-test-project-2", "idle", "never",
			},
			// The task id is shortened the way the other listings shorten it.
			wantAbsent: []string{occupiedTaskID},
		},
		{
			name:       "no Drudgers yet",
			pool:       nil,
			wantLines:  []string{"has no Drudgers"},
			wantAbsent: []string{"SLOT", "SANDBOX"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			setupProject(t)
			seedDrudgers(t, testCase.pool)

			var err error
			output := captureOutput(func() { err = drudgerList(nil) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

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
	setupHome(t)

	err := drudgerList(nil)
	if err == nil {
		t.Fatal("expected an error in a directory linked to no project")
	}
	if !strings.Contains(err.Error(), "no project initialized") {
		t.Errorf("expected the error to say no project is initialized, got %q", err)
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

func TestFormatLastChecked(t *testing.T) {
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
			got := formatLastChecked(testCase.lastChecked, now)
			if got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

// setupProject gives the test a home directory and a current directory linked
// to a project, which is what a Drudger command expects to find.
func setupProject(t *testing.T) {
	t.Helper()
	setupHome(t)

	var err error
	captureOutput(func() { err = projectInit([]string{testProjectName}) })
	if err != nil {
		t.Fatalf("could not initialize the test project: %v", err)
	}
}

// seedDrudgers writes a pool through the real repository, so the test reads
// back what a run would have left behind.
func seedDrudgers(t *testing.T, pool []*drudger.Drudger) {
	t.Helper()
	if len(pool) == 0 {
		return
	}

	repo := persistence.NewFileDrudgerRepository("")
	err := repo.UpdateDrudgers(testProjectSlug, func([]*drudger.Drudger) ([]*drudger.Drudger, error) {
		return pool, nil
	})
	if err != nil {
		t.Fatalf("could not seed the Drudgers: %v", err)
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
