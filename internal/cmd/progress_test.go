package cmd

import (
	"errors"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/git"
	"github.com/IgorBolotnikov/DRUDGE/internal/project"
	"github.com/IgorBolotnikov/DRUDGE/internal/release"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/theme"
)

func TestCLIProgress_Report(t *testing.T) {
	sampleTask := &task.Task{ID: "006684e3-dbe9-4316-8aba-8a67a8f01f8f", Title: "Fix login", Status: task.StatusTodo}
	fuckedUpTask := &task.Task{ID: sampleTask.ID, Title: sampleTask.Title, Status: task.StatusFuckedUp}
	doneTask := &task.Task{ID: sampleTask.ID, Title: sampleTask.Title, Status: task.StatusDone}
	refusedTask := &task.Task{
		ID:               sampleTask.ID,
		Title:            sampleTask.Title,
		Status:           task.StatusTodo,
		VendorErrorClass: task.VendorErrorAuth,
		VendorError:      "OAuth token has expired",
	}

	cases := []struct {
		name       string
		event      any
		want       string
		wantStderr string
	}{
		{
			name:  "a task created",
			event: task.TaskCreated{Task: sampleTask},
			want:  "✓ Created task 006684e3  Fix login\n",
		},
		{
			name:  "a declined removal",
			event: task.TaskRemovalDeclined{Task: sampleTask},
			want:  "· Left task 006684e3  Fix login alone\n",
		},
		{
			name:  "a removal started",
			event: task.TaskRemovalStarted{Task: sampleTask},
			want:  "Removing task 006684e3  Fix login\n",
		},
		{
			name:  "a run directory removed",
			event: task.RunDirectoryRemoved{TaskID: sampleTask.ID},
			want:  "✓ Removed its run directory\n",
		},
		{
			name:  "a task removed",
			event: task.TaskRemoved{Task: sampleTask},
			want:  "✓ Removed task 006684e3  Fix login\n",
		},
		{
			name:  "one task unblocked",
			event: task.TasksUnblocked{Count: 1},
			want:  "✓ Took it off the blockers of 1 task\n",
		},
		{
			name:  "several tasks unblocked",
			event: task.TasksUnblocked{Count: 2},
			want:  "✓ Took it off the blockers of 2 tasks\n",
		},
		{
			name:  "one task ungrouped",
			event: task.TasksUngrouped{Count: 1},
			want:  "✓ Ungrouped 1 task that belonged to it\n",
		},
		{
			name:  "several tasks ungrouped",
			event: task.TasksUngrouped{Count: 2},
			want:  "✓ Ungrouped 2 tasks that belonged to it\n",
		},
		{
			name:       "a branch cleanup after a removal failed",
			event:      task.BranchesCleanupFailed{TaskID: "3f9a1c2e", Err: errors.New("git said no")},
			wantStderr: "! Task 3f9a1c2e is removed, but the branches it left could not be cleaned up: git said no\n",
		},
		{
			name:       "a linked task that could not be written",
			event:      task.TaskUnlinkFailed{RemovedID: "3f9a1c2e", LinkedID: "4f2a1b3c", Err: errors.New("disk full")},
			wantStderr: "! Task 3f9a1c2e is removed, but task 4f2a1b3c still names it: disk full\n",
		},
		{
			name:       "a linked task another command holds",
			event:      task.TaskUnlinkFailed{RemovedID: "3f9a1c2e", LinkedID: "4f2a1b3c", IsHeld: true},
			wantStderr: "! Task 3f9a1c2e is removed, but another drudge command is working on task 4f2a1b3c, which still names it\n",
		},
		{
			name:  "a task marked done",
			event: task.TaskMarkedDone{Task: sampleTask},
			want:  `✓ Task 006684e3  Fix login is "todo"` + "\n",
		},
		{
			name:  "a task edited",
			event: task.TaskEdited{Task: sampleTask},
			want:  `✓ Updated task 006684e3  Fix login, it is now "todo"` + "\n",
		},
		{
			name:  "a project created",
			event: project.ProjectCreated{Project: &project.Project{Name: "Test Project"}},
			want:  "",
		},
		{
			name:  "a project renamed",
			event: project.ProjectRenamed{Slug: "demo", OldName: "Shop", NewName: "Store"},
			want:  `✓ Renamed project demo from "Shop" to "Store"` + "\n",
		},
		{
			name:  "a project removed",
			event: project.ProjectRemoved{Slug: "demo", Name: "Shop"},
			want:  "✓ Removed project Shop\n",
		},
		{
			name:  "a project already gone",
			event: project.ProjectAlreadyGone{Slug: "demo"},
			want:  "· Project demo was already gone\n",
		},
		{
			name:  "a download started",
			event: release.DownloadStarted{ArchiveName: "drg_linux_amd64.tar.gz", Version: "v0.2.0"},
			want:  "› Downloading drg_linux_amd64.tar.gz (v0.2.0)…\n",
		},
		{
			name:  "a Drudger claimed",
			event: drudger.DrudgerClaimed{Task: sampleTask, Slot: 3, Sandbox: "drudge-claude-demo-3"},
			want:  "Task 006684e3  Fix login → Drudger 3 (drudge-claude-demo-3)\n",
		},
		{
			name:  "a base fetch started",
			event: drudger.BaseFetchStarted{Repository: "api", Branch: "main", Remote: "origin"},
			want:  "› Fetching main of api from origin…\n",
		},
		{
			name:  "a worktree creation started",
			event: drudger.WorktreeCreationStarted{Repository: "api", Path: "/work/demo/.drudge/worktrees/slot-3/api"},
			want:  "› Creating the workspace of api at /work/demo/.drudge/worktrees/slot-3/api…\n",
		},
		{
			name:  "a worktree stashed",
			event: drudger.WorktreeStashed{Repository: "api", Commit: "9f1c2b3a4d5e6f7089a1b2c3d4e5f60718293a4b"},
			want:  "✓ The workspace of api held uncommitted changes, they are stashed at 9f1c2b3a4d5e\n",
		},
		{
			name:  "a branch checkout started",
			event: drudger.BranchCheckoutStarted{Branch: "drudge/006684e3-fix-login"},
			want:  "› Putting the workspace on branch drudge/006684e3-fix-login…\n",
		},
		{
			name:  "a sandbox lookup started",
			event: drudger.SandboxLookupStarted{Sandbox: "drudge-claude-demo-3"},
			want:  "› Looking for sandbox drudge-claude-demo-3…\n",
		},
		{
			name:  "a sandbox creation started",
			event: drudger.SandboxCreationStarted{Sandbox: "drudge-claude-demo-3", Timeout: 10 * time.Minute},
			want:  "› Creating sandbox drudge-claude-demo-3, the first one pulls its image, up to 10m0s…\n",
		},
		{
			name:  "a sandbox reused",
			event: drudger.SandboxReused{Sandbox: "drudge-claude-demo-3"},
			want:  "✓ Sandbox drudge-claude-demo-3 exists, reusing it\n",
		},
		{
			name:  "a line of sbx output",
			event: drudger.SbxOutput{Binary: "sbx", Line: "Pulling agent image 40%"},
			want:  "    sbx: Pulling agent image 40%\n",
		},
		{
			name:  "a line of sbx output with ANSI codes",
			event: drudger.SbxOutput{Binary: "sbx", Line: "\x1b[2K\x1b[32mPulling\x1b[0m agent image 40%"},
			want:  "    sbx: Pulling agent image 40%\n",
		},
		{
			name:       "an sbx daemon retried",
			event:      drudger.SbxDaemonRetried{},
			wantStderr: "! The sbx daemon did not come up, DRUDGE gives it one more try\n",
		},
		{
			name:  "an sbx daemon started",
			event: drudger.SbxDaemonStarted{},
			want:  "✓ The sbx daemon was not running, sbx started it\n",
		},
		{
			name:  "an agent launch started",
			event: drudger.AgentLaunchStarted{Sandbox: "drudge-claude-demo-3", GracePeriod: 10 * time.Second},
			want:  "› Starting the agent, waiting up to 10s for its first output…\n",
		},
		{
			name: "an agent launched",
			event: drudger.AgentLaunched{
				Task:    sampleTask,
				Sandbox: "drudge-claude-demo-3",
				Branch:  "drudge/006684e3-fix-login",
				RunDir:  "/work/demo/.drudge/runs/006684e3-dbe9-4316-8aba-8a67a8f01f8f",
			},
			want: "✓ Drudger drudge-claude-demo-3 is working on task 006684e3  Fix login\n" +
				"    Branch   drudge/006684e3-fix-login\n" +
				"    Run dir  /work/demo/.drudge/runs/006684e3-dbe9-4316-8aba-8a67a8f01f8f\n",
		},
		{
			name:  "a task restarted",
			event: drudger.TaskRestarted{Task: sampleTask, CameFrom: task.StatusFuckedUp},
			want:  `✓ Task 006684e3  Fix login was "fucked-up", it starts over` + "\n",
		},
		{
			name: "a run described",
			event: drudger.RunDescribed{
				Task:         sampleTask,
				Slot:         3,
				Sandbox:      "drudge-claude-demo-3",
				PromptSource: "~/.drudge/prompt.md",
				Prompt:       "Fix login\n\nSSO is broken since the last deploy.\n",
				Commands: [][]string{
					{"sbx", "ls", "--json"},
					{"sbx", "exec", "-d", "drudge-claude-demo-3"},
				},
			},
			want: "Dry run of task 006684e3  Fix login on Drudger 3 (drudge-claude-demo-3)\n" +
				"      Prompt from  ~/.drudge/prompt.md\n" +
				"\n" +
				"      Fix login\n" +
				"\n" +
				"      SSO is broken since the last deploy.\n" +
				"\n" +
				"      Commands\n" +
				"\n" +
				`      "sbx" "ls" "--json"` + "\n" +
				`      "sbx" "exec" "-d" "drudge-claude-demo-3"` + "\n" +
				"· Nothing ran, it was a dry run\n",
		},
		{
			name: "Drudgers above the limit",
			event: drudger.DrudgersAboveLimit{
				ProjectSlug: "demo",
				Limit:       2,
				Drudgers:    []*drudger.Drudger{{Slot: 3, Sandbox: "drudge-claude-demo-3"}, {Slot: 4, Sandbox: "drudge-claude-demo-4"}},
			},
			wantStderr: "! Project demo has Drudgers above the maxConcurrentDrudgers limit of 2, they get no tasks: slot 3 (drudge-claude-demo-3), slot 4 (drudge-claude-demo-4). " +
				"Raise maxConcurrentDrudgers to put them back to work, or nuke them if you are done with them\n",
		},
		{
			name:       "a Drudger list behind",
			event:      drudger.DrudgerListBehind{ProjectSlug: "demo"},
			wantStderr: "! Another drudge command holds the Drudgers of project demo, so this list is what was last written and may be behind\n",
		},
		{
			name:  "a Session recording started",
			event: drudger.SessionRecordingStarted{Task: sampleTask},
			want:  "Recording the Session of task 006684e3  Fix login\n",
		},
		{
			name:  "a Session that got shit done recorded",
			event: drudger.SessionRecorded{Task: doneTask, Status: drudger.StatusGotShitDone},
			want:  "✓ Task 006684e3  Fix login got shit done, it is done\n",
		},
		{
			name:  "a Session that needs babysitting recorded",
			event: drudger.SessionRecorded{Task: fuckedUpTask, Status: drudger.StatusNeedsBabysitting},
			want:  "! Task 006684e3  Fix login needs babysitting, it is fucked-up\n",
		},
		{
			name:  "a Session that fucked up recorded",
			event: drudger.SessionRecorded{Task: fuckedUpTask, Status: drudger.StatusFuckedUp},
			want:  "✗ Task 006684e3  Fix login fucked up, it is fucked-up\n",
		},
		{
			name:  "a Session that never got going recorded",
			event: drudger.SessionRecorded{Task: fuckedUpTask, Status: drudger.StatusNeverGotGoing},
			want:  "✗ Task 006684e3  Fix login never got going, it is fucked-up\n",
		},
		{
			name:  "a Session left unrecorded",
			event: drudger.SessionLeftUnrecorded{TaskID: sampleTask.ID},
			want:  "· Another drudge command is working on task 006684e3-dbe9-4316-8aba-8a67a8f01f8f, so this check reports the run directory without recording it\n",
		},
		{
			name:  "a run refused",
			event: drudger.RunRefused{Task: refusedTask},
			want: "! The vendor refused task 006684e3  Fix login (auth), it is back in todo\n" +
				"    Advice  Log in again, then re-seed the credentials inside sbx. sbx keeps its own copy of the token, and a host login does not refresh it.\n",
		},
		{
			name:  "work found on another branch",
			event: drudger.WorkFoundOnBranch{Repository: "api", Branch: "feature/sso"},
			want:  "✓ The agent left api on branch feature/sso, its work is there\n",
		},
		{
			name:  "a rescue branch created",
			event: drudger.RescueBranchCreated{Repository: "api", Branch: "drudge/006684e3-fix-login-rescue"},
			want:  "✓ The agent left api on no branch, its commits are on drudge/006684e3-fix-login-rescue\n",
		},
		{
			name:  "an empty branch dropped",
			event: drudger.EmptyBranchDropped{Repository: "api", Branch: "drudge/006684e3-fix-login"},
			want:  "· The agent committed nothing in api, branch drudge/006684e3-fix-login is deleted\n",
		},
		{
			name: "dependents unblocked",
			event: drudger.DependentsUnblocked{Tasks: []*task.Task{
				{ID: "4f2a1b3c-0001", Title: "Wire the repository"},
				{ID: "7e6d5c4b-0002", Title: "Reach 100% coverage"},
			}},
			want: "    Unblocked  4f2a1b3c  Wire the repository\n" +
				"               7e6d5c4b  Reach 100% coverage\n",
		},
		{
			name:       "a base fetch failed",
			event:      drudger.BaseFetchFailed{Repository: "api", Branch: "main", Err: errors.New("exit status 128")},
			wantStderr: "! Could not fetch main of api: exit status 128\n",
		},
		{
			name:       "a stale base used",
			event:      drudger.StaleBaseUsed{Repository: "api", Ref: "origin/main", Commit: git.Commit{SHA: "0123456789abcdef", CommittedAt: time.Now().Add(-73 * time.Hour)}},
			wantStderr: "! Work on api is cut from origin/main at 0123456789ab, committed 3 days ago\n",
		},
		{
			name:       "a stale base used that git does not resolve",
			event:      drudger.StaleBaseUsed{Repository: "api", Ref: "origin/main"},
			wantStderr: "! Work on api is cut from origin/main\n",
		},
		{
			name:       "a sandbox health record failed",
			event:      drudger.HealthRecordFailed{ProjectSlug: "demo", Part: drudger.SandboxPart, Slot: 2, Health: "gone", Err: errors.New("disk full")},
			wantStderr: "! The sandbox of Drudger 2 of project demo is gone, but that could not be recorded: disk full\n",
		},
		{
			name:       "a workspace health record failed",
			event:      drudger.HealthRecordFailed{ProjectSlug: "demo", Part: drudger.WorkspacePart, Slot: 2, Health: "misplaced", Err: errors.New("disk full")},
			wantStderr: "! The workspace of Drudger 2 of project demo is misplaced, but that could not be recorded: disk full\n",
		},
		{
			name:       "an agent health record failed",
			event:      drudger.HealthRecordFailed{ProjectSlug: "demo", Part: drudger.AgentPart, TaskID: "3f9a1c2e", Health: "refused", Err: errors.New("disk full")},
			wantStderr: "! The agent that ran task 3f9a1c2e of project demo is refused, but that could not be recorded: disk full\n",
		},
		{
			name:       "an idle workspace read failed",
			event:      drudger.IdleWorkspaceParkFailed{ProjectSlug: "demo", Slot: 2, Step: drudger.WorkspaceReadStep, Err: errors.New("no such file")},
			wantStderr: "! Drudger 2 of project demo holds no task, but the workspace it works in could not be read: no such file\n",
		},
		{
			name:       "an idle workspace park failed",
			event:      drudger.IdleWorkspaceParkFailed{ProjectSlug: "demo", Slot: 2, Step: drudger.WorkspaceParkStep, Err: errors.New("git said no")},
			wantStderr: "! Drudger 2 of project demo holds no task, but its workspace could not be parked: git said no\n",
		},
		{
			name:       "a run close-out workspace read failed",
			event:      drudger.RunCloseOutFailed{TaskID: "3f9a1c2e", Step: drudger.WorkspaceReadStep, Err: errors.New("no such file")},
			wantStderr: "! The Session of task 3f9a1c2e is over, but the workspace it ran in could not be read: no such file\n",
		},
		{
			name:       "a run close-out workspace park failed",
			event:      drudger.RunCloseOutFailed{TaskID: "3f9a1c2e", Step: drudger.WorkspaceParkStep, Err: errors.New("git said no")},
			wantStderr: "! The Session of task 3f9a1c2e is over, but the workspace it ran in could not be parked: git said no\n",
		},
		{
			name:       "a run close-out of a repository failed",
			event:      drudger.RunCloseOutFailed{TaskID: "3f9a1c2e", Step: drudger.RepositoryCloseOutStep, Repository: "api", Err: errors.New("git said no")},
			wantStderr: "! The Session of task 3f9a1c2e is over, but where its work in repository api is could not be worked out: git said no\n",
		},
		{
			name:       "a nuked workspace read failed",
			event:      drudger.WorkspaceNukeFailed{ProjectSlug: "demo", Slot: 2, Step: drudger.WorkspaceReadStep, Err: errors.New("no such file")},
			wantStderr: "! Drudger 2 of project demo is being nuked, but the workspace it works in could not be read: no such file\n",
		},
		{
			name:       "a nuked worktree removal failed",
			event:      drudger.WorkspaceNukeFailed{ProjectSlug: "demo", Slot: 2, Step: drudger.WorktreeRemovalStep, Repository: "api", Err: errors.New("git said no")},
			wantStderr: "! Drudger 2 of project demo is being nuked, but its worktree of repository api could not be taken out: git said no\n",
		},
		{
			name:       "a branch cleanup repository read failed",
			event:      drudger.BranchCleanupFailed{Repository: "api", Branch: "drudge/3f9a-add-retry", Step: drudger.RepositoryReadStep, Err: errors.New("no such file")},
			wantStderr: "! Could not read repository api, branch drudge/3f9a-add-retry stays: no such file\n",
		},
		{
			name:       "a branch cleanup branch read failed",
			event:      drudger.BranchCleanupFailed{Repository: "api", Branch: "drudge/3f9a-add-retry", Step: drudger.BranchReadStep, Err: errors.New("git said no")},
			wantStderr: "! Could not read branch drudge/3f9a-add-retry of repository api: git said no\n",
		},
		{
			name:       "a branch cleanup inspection failed",
			event:      drudger.BranchCleanupFailed{Repository: "api", Branch: "drudge/3f9a-add-retry", Step: drudger.BranchInspectStep, Err: errors.New("git said no")},
			wantStderr: "! Could not read what branch drudge/3f9a-add-retry of repository api holds: git said no\n",
		},
		{
			name:       "a branch cleanup delete failed",
			event:      drudger.BranchCleanupFailed{Repository: "api", Branch: "drudge/3f9a-add-retry", Step: drudger.BranchDeleteStep, Err: errors.New("git said no")},
			wantStderr: "! Could not delete branch drudge/3f9a-add-retry of repository api, it stays: git said no\n",
		},
		{
			name:       "an unblocked lookup failed",
			event:      drudger.UnblockedLookupFailed{TaskID: "3f9a1c2e", Err: errors.New("disk full")},
			wantStderr: "! Task 3f9a1c2e is done, but the tasks it unblocked could not be worked out: disk full\n",
		},
		{
			name:       "a Drudger release failed",
			event:      drudger.DrudgerReleaseFailed{ProjectSlug: "demo", Slot: 2, Err: errors.New("disk full")},
			wantStderr: "! Drudger 2 of project demo stays claimed for a run that never started: disk full\n",
		},
		{
			name:       "a session id read failed",
			event:      drudger.SessionIDReadFailed{TaskID: "3f9a1c2e", Err: errors.New("could not read the event stream of task 3f9a1c2e")},
			wantStderr: "! could not read the event stream of task 3f9a1c2e, the task is recorded without a session id\n",
		},
		{
			name:  "a Drudger nuke started",
			event: drudger.DrudgerNukeStarted{Slot: 3, Sandbox: "drudge-claude-demo-3"},
			want:  "Nuking Drudger 3 (drudge-claude-demo-3)\n",
		},
		{
			name:  "a Drudger nuked",
			event: drudger.DrudgerNuked{Slot: 3, Sandbox: "drudge-claude-demo-3"},
			want:  "✓ Drudger 3 is gone, sandbox drudge-claude-demo-3 was deleted\n",
		},
		{
			name:  "a sandbox already gone",
			event: drudger.SandboxAlreadyGone{Sandbox: "drudge-claude-demo-3"},
			want:  "· Sandbox drudge-claude-demo-3 was already gone\n",
		},
		{
			name:  "a task killed",
			event: drudger.TaskKilled{Task: fuckedUpTask},
			want:  "✗ Task 006684e3  Fix login is fucked-up, its agent was killed with the Drudger\n",
		},
		{
			name:  "a branch of an unknown repository kept",
			event: drudger.BranchOfUnknownRepositoryKept{ProjectSlug: "demo", Repository: "ui", Branch: "drudge/006684e3-fix-login"},
			want:  "· Branch drudge/006684e3-fix-login stays, project demo records no repository ui\n",
		},
		{
			name:  "a branch with commits kept",
			event: drudger.BranchWithCommitsKept{Repository: "api", Branch: "drudge/006684e3-fix-login"},
			want:  "· Branch drudge/006684e3-fix-login of repository api holds commits, it stays\n",
		},
		{
			name:  "an empty branch removed",
			event: drudger.EmptyBranchRemoved{Repository: "api", Branch: "drudge/006684e3-fix-login"},
			want:  "✓ Branch drudge/006684e3-fix-login of repository api held nothing, it is deleted\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")
			progress := newTestCLIProgress()
			var output string
			stderr := captureStderr(func() {
				output = captureOutput(func() { progress.Report(testCase.event) })
			})
			if output != testCase.want {
				t.Errorf("output = %q, want %q", output, testCase.want)
			}
			if stderr != testCase.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, testCase.wantStderr)
			}
		})
	}
}

func TestCLIProgress_ReportRunGroup(t *testing.T) {
	retryTask := &task.Task{ID: "3f9a1c2e-0b1d-4c2e-9f3a-1c2e0b1d4c2e", Title: "Add retry to uploader", Status: task.StatusTodo}
	claimed := drudger.DrudgerClaimed{Task: retryTask, Slot: 2, Sandbox: "drudge-demo-2"}
	fetchStarted := drudger.BaseFetchStarted{Repository: "api", Branch: "main", Remote: "origin"}
	worktreeStarted := drudger.WorktreeCreationStarted{Repository: "api", Path: ".drudge/worktrees/slot-2/api"}
	sandboxStarted := drudger.SandboxCreationStarted{Sandbox: "drudge-demo-2", Timeout: 10 * time.Minute}
	agentStarted := drudger.AgentLaunchStarted{Sandbox: "drudge-demo-2", GracePeriod: 30 * time.Second}
	launched := drudger.AgentLaunched{Task: retryTask, Sandbox: "drudge-demo-2", Branch: "drudge/3f9a-add-retry", RunDir: ".drudge/runs/3f9a1c2e"}

	const header = "Task 3f9a1c2e  Add retry to uploader → Drudger 2 (drudge-demo-2)\n"
	const result = "✓ Drudger drudge-demo-2 is working on task 3f9a1c2e  Add retry to uploader\n" +
		"    Branch   drudge/3f9a-add-retry\n" +
		"    Run dir  .drudge/runs/3f9a1c2e\n"

	cases := []struct {
		name       string
		events     []any
		want       string
		wantStderr string
	}{
		{
			name: "a run that works",
			events: []any{
				claimed,
				fetchStarted,
				worktreeStarted,
				sandboxStarted,
				drudger.SbxOutput{Binary: "sbx", Line: "pulling image docker/sandbox-claude:latest"},
				agentStarted,
				launched,
			},
			want: header +
				"  › Fetching main of api from origin…\n" +
				"  › Creating the workspace of api at .drudge/worktrees/slot-2/api…\n" +
				"  › Creating sandbox drudge-demo-2, the first one pulls its image, up to 10m0s…\n" +
				"      sbx: pulling image docker/sandbox-claude:latest\n" +
				"  › Starting the agent, waiting up to 30s for its first output…\n" +
				result,
		},
		{
			name: "a run whose fetch failed and whose sandbox did not come up",
			events: []any{
				claimed,
				fetchStarted,
				drudger.BaseFetchFailed{Repository: "api", Branch: "main", Err: errors.New("exit status 128")},
				drudger.StaleBaseUsed{Repository: "api", Ref: "origin/main"},
				worktreeStarted,
				sandboxStarted,
			},
			want: header +
				"  › Fetching main of api from origin…\n" +
				"  › Creating the workspace of api at .drudge/worktrees/slot-2/api…\n" +
				"  › Creating sandbox drudge-demo-2, the first one pulls its image, up to 10m0s…\n",
			wantStderr: "  ! Could not fetch main of api: exit status 128\n" +
				"  ! Work on api is cut from origin/main\n",
		},
		{
			name: "a run that reuses its sandbox",
			events: []any{
				claimed,
				drudger.WorktreeStashed{Repository: "api", Commit: "9f1c2b3a4d5e6f7089a1b2c3d4e5f60718293a4b"},
				drudger.BranchCheckoutStarted{Branch: "drudge/3f9a-add-retry"},
				drudger.SandboxLookupStarted{Sandbox: "drudge-demo-2"},
				drudger.SbxDaemonRetried{},
				drudger.SbxDaemonStarted{},
				drudger.SandboxReused{Sandbox: "drudge-demo-2"},
				agentStarted,
				launched,
			},
			want: header +
				"  ✓ The workspace of api held uncommitted changes, they are stashed at 9f1c2b3a4d5e\n" +
				"  › Putting the workspace on branch drudge/3f9a-add-retry…\n" +
				"  › Looking for sandbox drudge-demo-2…\n" +
				"  ✓ The sbx daemon was not running, sbx started it\n" +
				"  ✓ Sandbox drudge-demo-2 exists, reusing it\n" +
				"  › Starting the agent, waiting up to 30s for its first output…\n" +
				result,
			wantStderr: "  ! The sbx daemon did not come up, DRUDGE gives it one more try\n",
		},
		{
			name:   "a rerun prints the restart before the group",
			events: []any{drudger.TaskRestarted{Task: retryTask, CameFrom: task.StatusFuckedUp}, claimed, agentStarted, launched},
			want: `✓ Task 3f9a1c2e  Add retry to uploader was "fucked-up", it starts over` + "\n" +
				"\n" +
				header +
				"  › Starting the agent, waiting up to 30s for its first output…\n" +
				result,
		},
		{
			name: "a warning before the claim sits at column 0",
			events: []any{
				drudger.DrudgersAboveLimit{ProjectSlug: "demo", Limit: 1, Drudgers: []*drudger.Drudger{{Slot: 2, Sandbox: "drudge-demo-2"}}},
				claimed,
			},
			want: "\n" + header,
			wantStderr: "! Project demo has Drudgers above the maxConcurrentDrudgers limit of 1, they get no tasks: slot 2 (drudge-demo-2). " +
				"Raise maxConcurrentDrudgers to put them back to work, or nuke them if you are done with them\n",
		},
		{
			name: "a failed release sits in the group",
			events: []any{
				claimed,
				sandboxStarted,
				drudger.DrudgerReleaseFailed{ProjectSlug: "demo", Slot: 2, Err: errors.New("disk full")},
			},
			want: header +
				"  › Creating sandbox drudge-demo-2, the first one pulls its image, up to 10m0s…\n",
			wantStderr: "  ! Drudger 2 of project demo stays claimed for a run that never started: disk full\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")
			progress := newTestCLIProgress()
			var output string
			stderr := captureStderr(func() {
				output = captureOutput(func() {
					for _, event := range testCase.events {
						progress.Report(event)
					}
				})
			})
			if output != testCase.want {
				t.Errorf("output = %q, want %q", output, testCase.want)
			}
			if stderr != testCase.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, testCase.wantStderr)
			}
		})
	}
}

func TestCLIProgress_ReportNukeGroup(t *testing.T) {
	killedTask := &task.Task{ID: "3f9a1c2e-0b1d-4c2e-9f3a-1c2e0b1d4c2e", Title: "Add retry to uploader", Status: task.StatusFuckedUp}
	started := drudger.DrudgerNukeStarted{Slot: 2, Sandbox: "drudge-demo-2"}
	nuked := drudger.DrudgerNuked{Slot: 2, Sandbox: "drudge-demo-2"}

	const header = "Nuking Drudger 2 (drudge-demo-2)\n"
	const result = "✓ Drudger 2 is gone, sandbox drudge-demo-2 was deleted\n"

	cases := []struct {
		name       string
		events     []any
		want       string
		wantStderr string
	}{
		{
			name:   "an idle Drudger",
			events: []any{started, nuked},
			want:   header + result,
		},
		{
			name: "a forced Drudger whose sandbox was already gone",
			events: []any{
				started,
				drudger.SandboxAlreadyGone{Sandbox: "drudge-demo-2"},
				drudger.TaskKilled{Task: killedTask},
				nuked,
			},
			want: header +
				"  · Sandbox drudge-demo-2 was already gone\n" +
				"  ✗ Task 3f9a1c2e  Add retry to uploader is fucked-up, its agent was killed with the Drudger\n" +
				result,
		},
		{
			name: "a Drudger whose worktree could not be taken out",
			events: []any{
				started,
				drudger.WorkspaceNukeFailed{ProjectSlug: "demo", Slot: 2, Step: drudger.WorktreeRemovalStep, Repository: "api", Err: errors.New("git said no")},
				nuked,
			},
			want:       header + result,
			wantStderr: "  ! Drudger 2 of project demo is being nuked, but its worktree of repository api could not be taken out: git said no\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")
			progress := newTestCLIProgress()
			var output string
			stderr := captureStderr(func() {
				output = captureOutput(func() {
					for _, event := range testCase.events {
						progress.Report(event)
					}
				})
			})
			if output != testCase.want {
				t.Errorf("output = %q, want %q", output, testCase.want)
			}
			if stderr != testCase.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, testCase.wantStderr)
			}
		})
	}
}

func TestCLIProgress_ReportRecordingGroup(t *testing.T) {
	retryTask := &task.Task{ID: "3f9a1c2e-0b1d-4c2e-9f3a-1c2e0b1d4c2e", Title: "Add retry to uploader", Status: task.StatusDone}
	refusedTask := &task.Task{ID: retryTask.ID, Title: retryTask.Title, Status: task.StatusTodo, VendorErrorClass: task.VendorErrorRateLimit}

	const header = "Recording the Session of task 3f9a1c2e  Add retry to uploader\n"

	cases := []struct {
		name       string
		events     []any
		want       string
		wantStderr string
	}{
		{
			name: "a Session that got shit done and unblocked tasks",
			events: []any{
				drudger.SessionRecordingStarted{Task: retryTask},
				drudger.WorkFoundOnBranch{Repository: "api", Branch: "drudge/3f9a-add-retry"},
				drudger.EmptyBranchDropped{Repository: "web", Branch: "drudge/3f9a-add-retry"},
				drudger.SessionRecorded{Task: retryTask, Status: drudger.StatusGotShitDone},
				drudger.DependentsUnblocked{Tasks: []*task.Task{
					{ID: "77b04d1e-0001", Title: "Wire retry into the CLI"},
					{ID: "a01c9f3b-0002", Title: "Document retry flags"},
				}},
			},
			want: header +
				"  ✓ The agent left api on branch drudge/3f9a-add-retry, its work is there\n" +
				"  · The agent committed nothing in web, branch drudge/3f9a-add-retry is deleted\n" +
				"✓ Task 3f9a1c2e  Add retry to uploader got shit done, it is done\n" +
				"    Unblocked  77b04d1e  Wire retry into the CLI\n" +
				"               a01c9f3b  Document retry flags\n",
		},
		{
			name: "a close-out that failed sits in the group",
			events: []any{
				drudger.SessionRecordingStarted{Task: retryTask},
				drudger.RunCloseOutFailed{TaskID: retryTask.ID, Step: drudger.WorkspaceReadStep, Err: errors.New("disk full")},
				drudger.SessionRecorded{Task: retryTask, Status: drudger.StatusFuckedUp},
			},
			want: header +
				"✗ Task 3f9a1c2e  Add retry to uploader fucked up, it is done\n",
			wantStderr: "  ! The Session of task 3f9a1c2e-0b1d-4c2e-9f3a-1c2e0b1d4c2e is over, but the workspace it ran in could not be read: disk full\n",
		},
		{
			name: "a task marked done that unblocked tasks",
			events: []any{
				task.TaskMarkedDone{Task: retryTask},
				drudger.DependentsUnblocked{Tasks: []*task.Task{{ID: "77b04d1e-0001", Title: "Wire retry into the CLI"}}},
			},
			want: `✓ Task 3f9a1c2e  Add retry to uploader is "done"` + "\n" +
				"    Unblocked  77b04d1e  Wire retry into the CLI\n",
		},
		{
			name:   "a refused run sits at column 0 with its advice under it",
			events: []any{drudger.RunRefused{Task: refusedTask}},
			want: "! The vendor refused task 3f9a1c2e  Add retry to uploader (rate limit), it is back in todo\n" +
				"    Advice  Run the task again once the vendor lets you through.\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("NO_COLOR", "1")
			progress := newTestCLIProgress()
			var output string
			stderr := captureStderr(func() {
				output = captureOutput(func() {
					for _, event := range testCase.events {
						progress.Report(event)
					}
				})
			})
			if output != testCase.want {
				t.Errorf("output = %q, want %q", output, testCase.want)
			}
			if stderr != testCase.wantStderr {
				t.Errorf("stderr = %q, want %q", stderr, testCase.wantStderr)
			}
		})
	}
}

func newTestCLIProgress() *cliProgress {
	palette := theme.NewTheme(theme.DefaultTheme())
	return newCLIProgress(newPrinter(newThemedLogger(palette), palette))
}
