package cmd

import (
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/release"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestCLIProgress_Report(t *testing.T) {
	sampleTask := &task.Task{ID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f", Title: "Fix login", Status: task.StatusTodo}

	cases := []struct {
		name  string
		event any
		want  string
	}{
		{
			name:  "a task created",
			event: task.TaskCreated{Task: sampleTask},
			want:  "Created task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n",
		},
		{
			name:  "a declined removal",
			event: task.TaskRemovalDeclined{Task: sampleTask},
			want:  "Left task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login alone\n",
		},
		{
			name:  "a removal with no run directory",
			event: task.TaskRemoved{Task: sampleTask},
			want:  "Removed task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\n",
		},
		{
			name:  "a removal whose run directory went with it",
			event: task.TaskRemoved{Task: sampleTask, HasRun: true},
			want:  "Removed task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login\nIts run directory went with it\n",
		},
		{
			name:  "one task unblocked",
			event: task.TasksUnblocked{Count: 1},
			want:  "Took it off the blockers of 1 task\n",
		},
		{
			name:  "several tasks unblocked",
			event: task.TasksUnblocked{Count: 2},
			want:  "Took it off the blockers of 2 tasks\n",
		},
		{
			name:  "one task ungrouped",
			event: task.TasksUngrouped{Count: 1},
			want:  "Ungrouped 1 task that belonged to it\n",
		},
		{
			name:  "several tasks ungrouped",
			event: task.TasksUngrouped{Count: 2},
			want:  "Ungrouped 2 tasks that belonged to it\n",
		},
		{
			name:  "a task marked done",
			event: task.TaskMarkedDone{Task: sampleTask},
			want:  `Task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login is "todo"` + "\n",
		},
		{
			name:  "a task edited",
			event: task.TaskEdited{Task: sampleTask},
			want:  `Updated task [006684e3-dbe9-4316-8aba-8a67a8f01f8f] Fix login, it is now "todo"` + "\n",
		},
		{
			name:  "a project created",
			event: project.ProjectCreated{Project: &project.Project{Name: "Test Project"}},
			want:  "Created project Test Project\n",
		},
		{
			name:  "a project renamed",
			event: project.ProjectRenamed{Slug: "demo", OldName: "Shop", NewName: "Store"},
			want:  `Renamed project demo from "Shop" to "Store"` + "\n",
		},
		{
			name:  "a download started",
			event: release.DownloadStarted{ArchiveName: "drg_linux_amd64.tar.gz", Version: "v0.2.0"},
			want:  "Downloading drg_linux_amd64.tar.gz (v0.2.0)\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			progress := newCLIProgress(common.NewLogger(""))
			output := captureOutput(func() { progress.Report(testCase.event) })
			if output != testCase.want {
				t.Errorf("output = %q, want %q", output, testCase.want)
			}
		})
	}
}
