package cmd

import (
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestPrintTask(t *testing.T) {
	now := time.Date(2025, 3, 4, 12, 0, 0, 0, time.UTC)
	longDescription := strings.Repeat("The login form posts to the old endpoint. ", 20)

	cases := []struct {
		name     string
		task     task.Task
		blockers []task.Blocker
		family   task.Family
		unmerged map[task.TaskID][]drudger.UnmergedWork
		want     []string
		// wantAbsent is what the report must leave out for this task.
		wantAbsent []string
	}{
		{
			name: "a task no agent has had",
			task: task.Task{
				Status:      task.StatusTodo,
				TicketID:    "R-003-09",
				Description: longDescription,
				CreatedAt:   now.Add(-2 * time.Hour),
			},
			want:       []string{"todo", "R-003-09", longDescription, neverRunLabel, neverLabel},
			wantAbsent: []string{turnsLabel, costLabel, refusedLabel, blockedByLabel, parentLabel, childrenLabel},
		},
		{
			name: "a task an agent is working on",
			task: task.Task{
				Status:    task.StatusInProgress,
				CreatedAt: now.Add(-2 * time.Hour),
				StartedAt: now.Add(-30 * time.Minute),
				SessionID: "ebe60e03",
			},
			want:       []string{"in-progress", "ebe60e03", "30m ago", nothingReportedYetLabel},
			wantAbsent: []string{neverRunLabel, turnsLabel, costLabel},
		},
		{
			name: "a run that got shit done",
			task: task.Task{
				Status:          task.StatusDone,
				CreatedAt:       now.Add(-2 * time.Hour),
				StartedAt:       now.Add(-time.Hour),
				FinishedAt:      now.Add(-50 * time.Minute),
				SessionResult:   "Fixed the login",
				SessionTurns:    3,
				SessionDuration: 8664 * time.Millisecond,
				SessionCostUSD:  0.0695,
			},
			want:       []string{"done", "3", "9s", "$0.0695", "Fixed the login"},
			wantAbsent: []string{neverRunLabel, refusedLabel},
		},
		{
			name: "a run that fucked up",
			task: task.Task{
				Status:           task.StatusFuckedUp,
				CreatedAt:        now.Add(-2 * time.Hour),
				StartedAt:        now.Add(-time.Hour),
				FinishedAt:       now.Add(-50 * time.Minute),
				HasSessionFailed: true,
				SessionResult:    "could not find the login form",
				SessionTurns:     7,
				SessionDuration:  time.Minute,
				SessionCostUSD:   0.42,
			},
			want:       []string{"fucked-up", flaggedLabel, "7", "could not find the login form"},
			wantAbsent: []string{neverRunLabel, refusedLabel},
		},
		{
			name: "a run killed with its Drudger",
			task: task.Task{
				Status:     task.StatusFuckedUp,
				CreatedAt:  now.Add(-2 * time.Hour),
				StartedAt:  now.Add(-time.Hour),
				FinishedAt: now.Add(-50 * time.Minute),
			},
			want:       []string{"fucked-up", endedUnreportedLabel},
			wantAbsent: []string{neverRunLabel, turnsLabel, costLabel, flaggedLabel},
		},
		{
			name: "a run the vendor refused",
			task: task.Task{
				Status:           task.StatusTodo,
				CreatedAt:        now.Add(-2 * time.Hour),
				StartedAt:        now.Add(-time.Hour),
				VendorError:      "OAuth session expired",
				VendorErrorClass: task.VendorErrorAuth,
			},
			want:       []string{refusedLabel, string(task.VendorErrorAuth), "OAuth session expired"},
			wantAbsent: []string{neverRunLabel, turnsLabel, costLabel},
		},
		{
			name: "a run whose work landed in two repositories",
			task: task.Task{
				Status:     task.StatusDone,
				CreatedAt:  now.Add(-2 * time.Hour),
				StartedAt:  now.Add(-time.Hour),
				FinishedAt: now.Add(-50 * time.Minute),
				Landings: map[string]task.Landing{
					"api": {Branch: "drudge/006684e3", Base: "9f1c2b3a4d5e6f70", Head: "1a2b3c4d5e6f7081", Commits: 3},
					"ui":  {Branch: "drudge/006684e3", Base: "aaaaaaaaaaaaaaaa", Head: "bbbbbbbbbbbbbbbb", Commits: 1},
				},
			},
			want:       []string{workLabel, "api", "ui", "drudge/006684e3", "3 commits", "1 commit", "9f1c2b3a4d5e..1a2b3c4d5e6f"},
			wantAbsent: []string{committedNothingLabel},
		},
		{
			name: "a run that committed nothing",
			task: task.Task{
				Status:     task.StatusFuckedUp,
				CreatedAt:  now.Add(-2 * time.Hour),
				StartedAt:  now.Add(-time.Hour),
				FinishedAt: now.Add(-50 * time.Minute),
			},
			want: []string{workLabel, committedNothingLabel},
		},
		{
			name: "a run drudge has not closed out yet",
			task: task.Task{
				Status:    task.StatusInProgress,
				CreatedAt: now.Add(-2 * time.Hour),
				StartedAt: now.Add(-time.Hour),
				Landings: map[string]task.Landing{
					"api": {Branch: "drudge/006684e3", Base: "9f1c2b3a4d5e6f70"},
				},
			},
			want:       []string{workLabel, "drudge/006684e3"},
			wantAbsent: []string{"commits", committedNothingLabel},
		},
		{
			name: "a task blocked by other tasks",
			task: task.Task{
				Status:    task.StatusTodo,
				CreatedAt: now.Add(-2 * time.Hour),
				BlockedBy: []task.TaskID{"9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", "1a2b3c4d-dbe9-4316-8aba-8a67a8f01f8f"},
			},
			blockers: []task.Blocker{
				{
					ID:   "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f",
					Task: &task.Task{ID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Add the migration", Status: task.StatusDone},
				},
				{ID: "1a2b3c4d-dbe9-4316-8aba-8a67a8f01f8f"},
			},
			want: []string{
				"\n" + blockedByLabel + ":\n",
				"  9c8d7e6f  done         Add the migration\n",
				"  1a2b3c4d  " + missingTaskLabel + "\n",
			},
		},
		{
			name: "a task blocked by a done task with unmerged work",
			task: task.Task{
				Status:    task.StatusTodo,
				CreatedAt: now.Add(-2 * time.Hour),
				BlockedBy: []task.TaskID{"9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f"},
			},
			blockers: []task.Blocker{
				{
					ID:   "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f",
					Task: &task.Task{ID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Add the migration", Status: task.StatusDone},
				},
			},
			unmerged: map[task.TaskID][]drudger.UnmergedWork{
				"9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f": {
					{Repository: "drudge", Branch: "task/9c8d7e6f", BaseRef: "origin/main", Commits: 3},
					{Repository: "drudge-web", Branch: "task/9c8d7e6f", BaseRef: "origin/main", Commits: 1},
				},
			},
			want: []string{
				"  9c8d7e6f  done         Add the migration\n" +
					"    drudge      task/9c8d7e6f  3 commits not in origin/main\n" +
					"    drudge-web  task/9c8d7e6f  1 commit not in origin/main\n",
			},
		},
		{
			name: "a task belonging to another task",
			task: task.Task{
				Status:       task.StatusTodo,
				CreatedAt:    now.Add(-2 * time.Hour),
				ParentTaskID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f",
			},
			family: task.Family{
				ParentID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f",
				Parent:   &task.Task{ID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Dependency tracking", Status: task.StatusInProgress},
			},
			want: []string{
				"\n" + parentLabel + ":\n" +
					"  9c8d7e6f  in-progress  Dependency tracking\n",
			},
			wantAbsent: []string{childrenLabel},
		},
		{
			name: "a task whose parent is gone",
			task: task.Task{
				Status:       task.StatusTodo,
				CreatedAt:    now.Add(-2 * time.Hour),
				ParentTaskID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f",
			},
			family: task.Family{ParentID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f"},
			want: []string{
				"\n" + parentLabel + ":\n" +
					"  9c8d7e6f  " + missingTaskLabel + "\n",
			},
		},
		{
			name: "a task other tasks belong to",
			task: task.Task{
				Status:    task.StatusTodo,
				CreatedAt: now.Add(-2 * time.Hour),
			},
			family: task.Family{Children: []*task.Task{
				{ID: "2b3c4d5e-dbe9-4316-8aba-8a67a8f01f8f", Title: "Pick the next task", Status: task.StatusTodo},
				{ID: "7e6d5c4b-dbe9-4316-8aba-8a67a8f01f8f", Title: "Refuse a cycle", Status: task.StatusDone},
			}},
			want: []string{
				"\n" + childrenLabel + ":\n" +
					"  2b3c4d5e  todo         Pick the next task\n" +
					"  7e6d5c4b  done         Refuse a cycle\n",
			},
			wantAbsent: []string{parentLabel},
		},
		{
			name: "a task carrying no ticket and no description",
			task: task.Task{
				Status:    task.StatusDraft,
				CreatedAt: now.Add(-2 * time.Hour),
			},
			want: []string{"draft", noneLabel},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			taskToShow := testCase.task
			taskToShow.ID = "006684e3-dbe9-4316-8aba-8a67a8f01f8f"
			taskToShow.Title = "Fix login"
			log := common.NewLogger("")

			out := captureOutput(func() { printTask(log, &taskToShow, testCase.blockers, testCase.unmerged, testCase.family, now) })

			for _, want := range append(testCase.want, string(taskToShow.ID), taskToShow.Title) {
				if !strings.Contains(out, want) {
					t.Errorf("expected the report to mention %q, got:\n%s", want, out)
				}
			}
			for _, absent := range testCase.wantAbsent {
				if strings.Contains(out, absent) {
					t.Errorf("expected the report to leave out %q, got:\n%s", absent, out)
				}
			}
			if !strings.Contains(out, "\n"+lastRunLabel+":") {
				t.Errorf("expected %q to head its section without an indent, got:\n%s", lastRunLabel, out)
			}
		})
	}
}
