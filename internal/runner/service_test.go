package runner

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

const (
	testProjectSlug = "test-project"

	// The sandbox drudge names for the first two runner slots of the test
	// project, under the default harness.
	testSandbox      = "drudge-claude-test-project-1"
	testSandboxSlot2 = "drudge-claude-test-project-2"

	// runWorkspace stands in for the workspace a run happens in, so a table
	// can name that path before the test has one.
	runWorkspace = "{{workspace}}"
)

type fakeTaskRepo struct {
	tasks []*task.Task
}

func (repo *fakeTaskRepo) CreateTask(dto task.CreateTaskDto) (*task.Task, error) {
	return nil, fmt.Errorf("CreateTask should not be called")
}

func (repo *fakeTaskRepo) ListTasks(projectSlug string) ([]*task.Task, error) {
	return repo.tasks, nil
}

func (repo *fakeTaskRepo) FindTask(projectSlug string, fullOrPartialID string) (*task.Task, error) {
	for _, candidate := range repo.tasks {
		if strings.HasPrefix(string(candidate.ID), fullOrPartialID) {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("task %q not found", fullOrPartialID)
}

func (repo *fakeTaskRepo) UpdateTask(projectSlug string, taskToUpdate *task.Task) error {
	for index, candidate := range repo.tasks {
		if candidate.ID == taskToUpdate.ID {
			taskToUpdate.UpdatedAt = time.Now().UTC()
			repo.tasks[index] = taskToUpdate
			return nil
		}
	}
	return fmt.Errorf("task %q not found", taskToUpdate.ID)
}

// fakeCommandRunner answers a fixed script of calls and remembers what it was
// asked to run, in order. It swaps its workspace into the runWorkspace
// placeholder of every output it hands back.
type fakeCommandRunner struct {
	workspace string
	calls     [][]string
	outputs   []string
	errs      []error
}

func (runner *fakeCommandRunner) Run(argv []string) (string, error) {
	index := len(runner.calls)
	runner.calls = append(runner.calls, argv)

	var output string
	if index < len(runner.outputs) {
		output = inWorkspace(runner.outputs[index], runner.workspace)
	}
	var err error
	if index < len(runner.errs) {
		err = runner.errs[index]
	}
	return output, err
}

// subcommands names the sbx subcommand of every call, in order, so a test can
// assert on the shape of a run without repeating whole argvs.
func (runner *fakeCommandRunner) subcommands() []string {
	names := make([]string, 0, len(runner.calls))
	for _, argv := range runner.calls {
		names = append(names, argv[1])
	}
	return names
}

// call returns the first call to an sbx subcommand, or nil if it never came.
func (runner *fakeCommandRunner) call(subcommand string) []string {
	for _, argv := range runner.calls {
		if argv[1] == subcommand {
			return argv
		}
	}
	return nil
}

func newTestService(tasks ...*task.Task) *RunnerService {
	return newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), &fakeCommandRunner{}, tasks...)
}

func newTestServiceWith(localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, commands CommandRunner, tasks ...*task.Task) *RunnerService {
	logger := common.NewLogger("")
	return New(logger, localCfg, globalCfg, task.NewTaskService(&fakeTaskRepo{tasks: tasks}, logger), commands)
}

// setupWorkspace moves the test into a temp workspace and returns its path.
func setupWorkspace(t *testing.T) string {
	return setupWorkspaceNamed(t, "")
}

// setupWorkspaceNamed moves the test into a named directory of a temp
// workspace, so a test can pick a path a shell would choke on unquoted.
func setupWorkspaceNamed(t *testing.T, name string) string {
	t.Helper()
	setupPromptDirs(t)

	if name != "" {
		if err := common.EnsureDir(name); err != nil {
			t.Fatalf("could not create the workspace directory: %v", err)
		}
		if err := os.Chdir(name); err != nil {
			t.Fatalf("Chdir: %v", err)
		}
	}

	workspace, err := common.WorkDir()
	if err != nil {
		t.Fatalf("could not resolve the workspace: %v", err)
	}
	return workspace
}

// todoTask is the task every run test hands to an agent.
func todoTask() *task.Task {
	return &task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	}
}

// busyTask holds a runner slot so the next run has to allocate another one.
func busyTask(runnerID int) *task.Task {
	return &task.Task{
		ID:          task.TaskID(fmt.Sprintf("busy-%d", runnerID)),
		Title:       "Already running",
		Status:      task.StatusInProgress,
		RunnerID:    runnerID,
		ProjectSlug: testProjectSlug,
	}
}

// inWorkspace swaps a workspace into the runWorkspace placeholder of a value.
func inWorkspace(value, workspace string) string {
	return strings.ReplaceAll(value, runWorkspace, workspace)
}

// sandboxListingWith renders the `sbx ls --json` output for a listing that
// holds exactly the named sandboxes, each mounted on the workspace of the run.
func sandboxListingWith(names ...string) string {
	entries := make([]string, 0, len(names))
	for _, name := range names {
		entries = append(entries, sandboxEntry(name, runWorkspace))
	}
	return sandboxListingOf(entries...)
}

// sandboxListingMountedOn renders a listing holding one sandbox with the given
// mounts, so a test can point it away from the workspace of the run.
func sandboxListingMountedOn(name string, mounts ...string) string {
	return sandboxListingOf(sandboxEntry(name, mounts...))
}

// sandboxEntry renders one sandbox of a listing.
func sandboxEntry(name string, mounts ...string) string {
	quoted := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		quoted = append(quoted, fmt.Sprintf("%q", mount))
	}
	return fmt.Sprintf(`{"name":%q,"workspaces":[%s]}`, name, strings.Join(quoted, ","))
}

// sandboxListingOf wraps rendered sandbox entries in a listing.
func sandboxListingOf(entries ...string) string {
	return fmt.Sprintf(`{"sandboxes":[%s]}`, strings.Join(entries, ","))
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

func TestRunnerService_RunTask_OnlyRunsTodoTasks(t *testing.T) {
	cases := []struct {
		name    string
		status  task.TaskStatus
		wantErr bool
	}{
		{name: "runs a todo task", status: task.StatusTodo},
		{name: "refuses a draft task", status: task.StatusDraft, wantErr: true},
		{name: "refuses an in-progress task", status: task.StatusInProgress, wantErr: true},
		{name: "refuses a fucked-up task", status: task.StatusFuckedUp, wantErr: true},
		{name: "refuses a done task", status: task.StatusDone, wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			taskToRun := todoTask()
			taskToRun.Status = testCase.status
			service := newTestService(taskToRun)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, true) })

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error for status %q", testCase.status)
				}
				if !strings.Contains(err.Error(), string(testCase.status)) {
					t.Errorf("expected error to name status %q, got %q", testCase.status, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunnerService_RunTask_UnknownTask(t *testing.T) {
	service := newTestService()

	err := service.RunTask(testProjectSlug, "nope", true)
	if err == nil {
		t.Fatal("expected an error for an unknown task ID")
	}
}

func TestRunnerService_RunTask_RecordsTheRunnerOnTheTask(t *testing.T) {
	workspace := setupWorkspace(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(testSandbox)}}
	service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if taskToRun.Status != task.StatusInProgress {
		t.Errorf("expected status %q, got %q", task.StatusInProgress, taskToRun.Status)
	}
	if taskToRun.RunnerID != 1 {
		t.Errorf("expected runner 1, got %d", taskToRun.RunnerID)
	}
	if taskToRun.StartedAt.IsZero() {
		t.Error("expected started at to be stamped")
	}
	// The session id only exists once the agent has written its init event, so
	// a task is recorded without one.
	if taskToRun.RunnerSessionID != "" {
		t.Errorf("expected no session id to be recorded, got %q", taskToRun.RunnerSessionID)
	}
}

func TestRunnerService_RunTask_CreatesTheSandboxOnlyWhenItIsMissing(t *testing.T) {
	createThenStart := []string{sbxLsSubcommand, sbxCreateSubcommand, sbxExecSubcommand}
	startOnly := []string{sbxLsSubcommand, sbxExecSubcommand}

	cases := []struct {
		name    string
		listing string
		want    []string
	}{
		{name: "an empty listing", listing: sandboxListingWith(), want: createThenStart},
		{name: "a listing without the key", listing: `{}`, want: createThenStart},
		{name: "another project's sandbox", listing: sandboxListingWith("drudge-claude-other-project-1"), want: createThenStart},
		{name: "another slot of this project", listing: sandboxListingWith(testSandboxSlot2), want: createThenStart},
		{name: "this runner's sandbox", listing: sandboxListingWith(testSandbox), want: startOnly},
		{name: "this runner's sandbox among others", listing: sandboxListingWith("drudge-claude-other-project-1", testSandbox), want: startOnly},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{workspace: workspace, outputs: []string{testCase.listing}}
			service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got := commands.subcommands(); !slices.Equal(got, testCase.want) {
				t.Errorf("expected sbx calls %v, got %v", testCase.want, got)
			}
		})
	}
}

func TestRunnerService_RunTask_RefusesASandboxHoldingAnotherWorkspace(t *testing.T) {
	const otherRepo = "/some/other/repo"

	cases := []struct {
		name    string
		mounts  []string
		wantErr bool
	}{
		{name: "the workspace of the run", mounts: []string{runWorkspace}},
		{name: "a trailing slash is the same path", mounts: []string{runWorkspace + "/"}},
		{name: "the workspace among several mounts", mounts: []string{otherRepo, runWorkspace}},
		{name: "another repository", mounts: []string{otherRepo}, wantErr: true},
		{name: "several mounts, none of them the workspace", mounts: []string{otherRepo, "/yet/another"}, wantErr: true},
		{name: "a path the workspace is only a prefix of", mounts: []string{runWorkspace + "-old"}, wantErr: true},
		{name: "no workspace at all", wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{
				workspace: workspace,
				outputs:   []string{sandboxListingMountedOn(testSandbox, testCase.mounts...)},
			}
			service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })

			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected the workspace mismatch to surface")
				}
				for _, want := range append([]string{testSandbox, workspace}, testCase.mounts...) {
					named := inWorkspace(want, workspace)
					if !strings.Contains(err.Error(), named) {
						t.Errorf("expected the error to name %q, got %q", named, err)
					}
				}
				if got := commands.subcommands(); !slices.Equal(got, []string{sbxLsSubcommand}) {
					t.Errorf("expected nothing to run past the listing, got %v", got)
				}
				if taskToRun.Status != task.StatusTodo {
					t.Errorf("expected the task to stay %q, got %q", task.StatusTodo, taskToRun.Status)
				}
				if taskToRun.RunnerID != 0 {
					t.Errorf("expected no runner to be recorded, got %d", taskToRun.RunnerID)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestRunnerService_RunTask_WritesThePromptForTheAgentToRead(t *testing.T) {
	workspace := setupWorkspace(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(testSandbox)}}
	service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	runDir := common.RunDir(workspace, string(taskToRun.ID))
	prompt, err := common.ReadFile(common.RunPromptPath(runDir))
	if err != nil {
		t.Fatalf("expected the prompt to be written to the run directory: %v", err)
	}
	for _, want := range []string{taskToRun.Title, taskToRun.Description} {
		if !strings.Contains(prompt, want) {
			t.Errorf("expected the prompt file to contain %q, got %q", want, prompt)
		}
	}
}

func TestRunnerService_RunTask_StepFailureLeavesTheTaskAlone(t *testing.T) {
	spawnErr := fmt.Errorf("sbx: no such binary")

	cases := []struct {
		name string
		// A run directory survives only a failure that happens after the
		// sandbox is known to be up. Anything earlier leaves nothing behind.
		outputs    []string
		errs       []error
		wantRuns   int
		wantRunDir bool
	}{
		{
			name:     "the listing fails",
			errs:     []error{spawnErr},
			wantRuns: 1,
		},
		{
			name:     "the listing cannot be parsed",
			outputs:  []string{"sbx: command not found"},
			wantRuns: 1,
		},
		{
			name:     "creating the sandbox fails",
			outputs:  []string{sandboxListingWith()},
			errs:     []error{nil, spawnErr},
			wantRuns: 2,
		},
		{
			name:       "starting the agent fails",
			outputs:    []string{sandboxListingWith(testSandbox)},
			errs:       []error{nil, spawnErr},
			wantRuns:   2,
			wantRunDir: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{workspace: workspace, outputs: testCase.outputs, errs: testCase.errs}
			service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			if err == nil {
				t.Fatal("expected the failure to surface")
			}

			if len(commands.calls) != testCase.wantRuns {
				t.Errorf("expected %d sbx calls, got %v", testCase.wantRuns, commands.subcommands())
			}
			if taskToRun.Status != task.StatusTodo {
				t.Errorf("expected the task to stay %q, got %q", task.StatusTodo, taskToRun.Status)
			}
			if taskToRun.RunnerID != 0 {
				t.Errorf("expected no runner to be recorded, got %d", taskToRun.RunnerID)
			}

			exists, err := common.Exists(common.RunDir(workspace, string(taskToRun.ID)))
			if err != nil {
				t.Fatalf("could not check the run directory: %v", err)
			}
			if exists != testCase.wantRunDir {
				t.Errorf("expected the run directory to exist %t, got %t", testCase.wantRunDir, exists)
			}
		})
	}
}

func TestRunnerService_RunTask_AllocatesTheLowestFreeRunnerSlot(t *testing.T) {
	cases := []struct {
		name    string
		limit   int
		busy    []*task.Task
		wantID  int
		wantErr bool
	}{
		{name: "empty pool takes the first slot", limit: 3, wantID: 1},
		{name: "takes the next free slot", limit: 3, busy: []*task.Task{busyTask(1)}, wantID: 2},
		{name: "fills a gap left in the middle", limit: 3, busy: []*task.Task{busyTask(1), busyTask(3)}, wantID: 2},
		{
			name:  "ignores tasks that are not in progress",
			limit: 3,
			busy: []*task.Task{
				{ID: "done", Status: task.StatusDone, RunnerID: 1, ProjectSlug: testProjectSlug},
				{ID: "fucked-up", Status: task.StatusFuckedUp, RunnerID: 2, ProjectSlug: testProjectSlug},
			},
			wantID: 1,
		},
		{
			name:    "fails when every slot is taken",
			limit:   2,
			busy:    []*task.Task{busyTask(1), busyTask(2)},
			wantErr: true,
		},
		{
			name:    "ignores slots above the limit but still fails when full",
			limit:   1,
			busy:    []*task.Task{busyTask(1), busyTask(7)},
			wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWith(
				&config.LocalConfig{ProjectSlug: testProjectSlug, MaxConcurrentRunners: testCase.limit},
				config.DefaultConfig(),
				commands,
				append([]*task.Task{taskToRun}, testCase.busy...)...,
			)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got runner %d", taskToRun.RunnerID)
				}
				if !strings.Contains(err.Error(), config.MaxConcurrentRunnersKey) {
					t.Errorf("expected the error to name the config key, got %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if taskToRun.RunnerID != testCase.wantID {
				t.Errorf("expected runner %d, got %d", testCase.wantID, taskToRun.RunnerID)
			}
			wantSandbox := fmt.Sprintf("drudge-claude-%s-%d", testProjectSlug, testCase.wantID)
			if create := commands.call(sbxCreateSubcommand); !slices.Contains(create, wantSandbox) {
				t.Errorf("expected the create call to name sandbox %q, got %v", wantSandbox, create)
			}
		})
	}
}

func TestRunnerService_RunTask_UsesTheConfiguredPromptFile(t *testing.T) {
	const promptFileName = "impl.md"
	setupWorkspace(t)
	writePromptFile(t, common.LocalPromptsDir(), promptFileName, "custom prompt for {{taskTitle}}: {{taskDescription}}")

	taskToRun := todoTask()
	service := newTestServiceWith(
		&config.LocalConfig{ProjectSlug: testProjectSlug, PromptFile: promptFileName},
		config.DefaultConfig(),
		&fakeCommandRunner{},
		taskToRun,
	)

	var err error
	out := captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, true) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{"custom prompt for Fix login: SSO is broken", promptFileName} {
		if !strings.Contains(out, want) {
			t.Errorf("expected dry run output to contain %q, got %q", want, out)
		}
	}
}

func TestRunnerService_RunTask_PromptFileMissingPlaceholderNamesTheFile(t *testing.T) {
	const promptFileName = "impl.md"
	setupWorkspace(t)
	writePromptFile(t, common.LocalPromptsDir(), promptFileName, "nothing to substitute here")

	service := newTestServiceWith(
		&config.LocalConfig{ProjectSlug: testProjectSlug, PromptFile: promptFileName},
		config.DefaultConfig(),
		&fakeCommandRunner{},
		todoTask(),
	)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, "task-1", true) })
	if err == nil {
		t.Fatal("expected an error for a prompt file without the required placeholders")
	}
	for _, want := range []string{promptFileName, placeholderTaskTitle} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected error to name %s, got %q", want, err)
		}
	}
}
