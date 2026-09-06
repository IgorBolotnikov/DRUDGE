package drudger

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

	testSandbox      = "drudge-claude-test-project-1"
	testSandboxSlot2 = "drudge-claude-test-project-2"

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

func (repo *fakeTaskRepo) GetTask(projectSlug string, id task.TaskID) (*task.Task, error) {
	for _, candidate := range repo.tasks {
		if candidate.ID == id {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("task %q not found", id)
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
//
// A started command is remembered a second time, since a command drudge does
// not wait for is a different thing from one it does.
type fakeCommandRunner struct {
	workspace string
	calls     [][]string
	started   [][]string
	outputs   []string
	errs      []error
}

func (runner *fakeCommandRunner) Start(argv []string) error {
	runner.started = append(runner.started, argv)
	_, err := runner.Run(argv)
	return err
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

func (runner *fakeCommandRunner) subcommands() []string {
	names := make([]string, 0, len(runner.calls))
	for _, argv := range runner.calls {
		names = append(names, argv[1])
	}
	return names
}

// startedSubcommands names the sbx subcommands drudge started without waiting.
func (runner *fakeCommandRunner) startedSubcommands() []string {
	names := make([]string, 0, len(runner.started))
	for _, argv := range runner.started {
		names = append(names, argv[1])
	}
	return names
}

// call returns the first call to an sbx subcommand.
func (runner *fakeCommandRunner) call(subcommand string) []string {
	for _, argv := range runner.calls {
		if argv[1] == subcommand {
			return argv
		}
	}
	return nil
}

// fakeDrudgerRepo keeps project's Drudgers in memory and mimics the behavior
// of a real repo.
type fakeDrudgerRepo struct {
	drudgers []*Drudger
}

func (repo *fakeDrudgerRepo) ListDrudgers(projectSlug string) ([]*Drudger, error) {
	return copyDrudgers(repo.drudgers), nil
}

func (repo *fakeDrudgerRepo) UpdateDrudgers(projectSlug string, change func([]*Drudger) ([]*Drudger, error)) error {
	updated, err := change(copyDrudgers(repo.drudgers))
	if err != nil {
		return err
	}
	repo.drudgers = updated
	return nil
}

// holderOf returns the Drudger occupied by a task, or nil if none.
func (repo *fakeDrudgerRepo) holderOf(taskID task.TaskID) *Drudger {
	for _, candidate := range repo.drudgers {
		if candidate.TaskID == taskID {
			return candidate
		}
	}
	return nil
}

// atSlot returns the Drudger of a slot, or nil when the slot has none.
func (repo *fakeDrudgerRepo) atSlot(slot int) *Drudger {
	for _, candidate := range repo.drudgers {
		if candidate.Slot == slot {
			return candidate
		}
	}
	return nil
}

func copyDrudgers(drudgers []*Drudger) []*Drudger {
	copied := make([]*Drudger, 0, len(drudgers))
	for _, candidate := range drudgers {
		duplicate := *candidate
		copied = append(copied, &duplicate)
	}
	return copied
}

type testService struct {
	*DrudgerService
	drudgers *fakeDrudgerRepo
}

func newTestService(tasks ...*task.Task) *testService {
	return newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), &fakeCommandRunner{}, tasks...)
}

func newTestServiceWith(localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, commands CommandRunner, tasks ...*task.Task) *testService {
	return newTestServiceWithPool(localCfg, globalCfg, commands, nil, tasks...)
}

func newTestServiceWithPool(localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, commands CommandRunner, pool []*Drudger, tasks ...*task.Task) *testService {
	logger := common.NewLogger("")
	drudgers := &fakeDrudgerRepo{drudgers: pool}
	return &testService{
		DrudgerService: New(logger, localCfg, globalCfg, task.NewTaskService(&fakeTaskRepo{tasks: tasks}, logger), drudgers, commands),
		drudgers:       drudgers,
	}
}

func setupWorkspace(t *testing.T) string {
	return setupWorkspaceNamed(t, "")
}

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

func todoTask() *task.Task {
	return &task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	}
}

func busyDrudger(slot int) *Drudger {
	return &Drudger{
		Slot:    slot,
		Sandbox: testSandboxOfSlot(slot),
		TaskID:  busyTaskID(slot),
	}
}

func idleDrudger(slot int) *Drudger {
	return &Drudger{Slot: slot, Sandbox: testSandboxOfSlot(slot)}
}

func busyTaskID(slot int) task.TaskID {
	return task.TaskID(fmt.Sprintf("busy-%d", slot))
}

func testSandboxOfSlot(slot int) string {
	return fmt.Sprintf("drudge-claude-%s-%d", testProjectSlug, slot)
}

func finishSession(t *testing.T, workspace string, taskID task.TaskID) {
	t.Helper()
	writeExit(t, common.RunDir(workspace, string(taskID)), "0\n")
}

func inWorkspace(value, workspace string) string {
	return strings.ReplaceAll(value, runWorkspace, workspace)
}

func sandboxListingWith(names ...string) string {
	entries := make([]string, 0, len(names))
	for _, name := range names {
		entries = append(entries, sandboxEntry(name, runWorkspace))
	}
	return sandboxListingOf(entries...)
}

func sandboxListingMountedOn(name string, mounts ...string) string {
	return sandboxListingOf(sandboxEntry(name, mounts...))
}

func sandboxEntry(name string, mounts ...string) string {
	quoted := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		quoted = append(quoted, fmt.Sprintf("%q", mount))
	}
	return fmt.Sprintf(`{"name":%q,"workspaces":[%s]}`, name, strings.Join(quoted, ","))
}

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

func TestDrudgerService_RunTask_OnlyRunsTodoTasks(t *testing.T) {
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

func TestDrudgerService_RunTask_UnknownTask(t *testing.T) {
	service := newTestService()

	err := service.RunTask(testProjectSlug, "nope", true)
	if err == nil {
		t.Fatal("expected an error for an unknown task ID")
	}
}

func TestDrudgerService_RunTask_RecordsTheClaimOnTheDrudger(t *testing.T) {
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
	if taskToRun.StartedAt.IsZero() {
		t.Error("expected started at to be stamped")
	}

	claimed := service.drudgers.holderOf(taskToRun.ID)
	if claimed == nil {
		t.Fatalf("expected a Drudger to hold the task, got pool %v", service.drudgers.drudgers)
	}
	if claimed.Slot != 1 {
		t.Errorf("expected Drudger 1, got %d", claimed.Slot)
	}
	if claimed.Sandbox != testSandbox {
		t.Errorf("expected sandbox %q, got %q", testSandbox, claimed.Sandbox)
	}
	if claimed.LastChecked.IsZero() {
		t.Error("expected last checked to be stamped")
	}
	if taskToRun.SessionID != "" {
		t.Errorf("expected no session id to be recorded, got %q", taskToRun.SessionID)
	}
}

func TestDrudgerService_RunTask_RecordsTheSessionIDTheAgentHasWritten(t *testing.T) {
	cases := []struct {
		name string
		// lines stand for what the agent has written to its stream by the time
		// the launch returns. Nothing there is the usual case, since an agent
		// takes a moment to start up.
		lines []string
		want  string
	}{
		{name: "the agent has not started writing yet"},
		{name: "the agent has written its init event", lines: []string{initEvent}, want: sampleSessionID},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			if testCase.lines != nil {
				writeStream(t, common.RunDir(workspace, string(taskToRun.ID)), testCase.lines...)
			}

			commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(testSandbox)}}
			service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if taskToRun.SessionID != testCase.want {
				t.Errorf("expected session id %q, got %q", testCase.want, taskToRun.SessionID)
			}
			if taskToRun.Status != task.StatusInProgress {
				t.Errorf("expected status %q, got %q", task.StatusInProgress, taskToRun.Status)
			}
		})
	}
}

func TestDrudgerService_RunTask_CreatesTheSandboxOnlyWhenItIsMissing(t *testing.T) {
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
		{name: "this Drudger's sandbox", listing: sandboxListingWith(testSandbox), want: startOnly},
		{name: "this Drudger's sandbox among others", listing: sandboxListingWith("drudge-claude-other-project-1", testSandbox), want: startOnly},
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

func TestDrudgerService_RunTask_RefusesASandboxHoldingAnotherWorkspace(t *testing.T) {
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
				if held := service.drudgers.holderOf(taskToRun.ID); held != nil {
					t.Errorf("expected no Drudger to hold the task, got Drudger %d", held.Slot)
				}
				if recorded := service.drudgers.atSlot(1); recorded.Health != HealthMisplaced {
					t.Errorf("expected the refusal to be recorded as %q, got %q", HealthMisplaced, recorded.Health)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if recorded := service.drudgers.atSlot(1); recorded.Health != HealthUsable {
				t.Errorf("expected the sandbox to be recorded as %q, got %q", HealthUsable, recorded.Health)
			}
		})
	}
}

func TestDrudgerService_RunTask_WritesThePromptForTheAgentToRead(t *testing.T) {
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

func TestDrudgerService_RunTask_StepFailureLeavesTheTaskAlone(t *testing.T) {
	spawnErr := fmt.Errorf("sbx: no such binary")

	cases := []struct {
		name       string
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
			if held := service.drudgers.holderOf(taskToRun.ID); held != nil {
				t.Errorf("expected the claim to be released, got Drudger %d", held.Slot)
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

func TestDrudgerService_RunTask_AllocatesTheLowestFreeDrudgerSlot(t *testing.T) {
	cases := []struct {
		name     string
		limit    int
		pool     []*Drudger
		finished []task.TaskID
		wantSlot int
		wantErr  bool
	}{
		{name: "empty pool takes the first slot", limit: 3, wantSlot: 1},
		{name: "takes the next free slot", limit: 3, pool: []*Drudger{busyDrudger(1)}, wantSlot: 2},
		{name: "fills a gap left in the middle", limit: 3, pool: []*Drudger{busyDrudger(1), busyDrudger(3)}, wantSlot: 2},
		{name: "reuses a Drudger nobody is working in", limit: 3, pool: []*Drudger{idleDrudger(1)}, wantSlot: 1},
		{
			name:     "reclaims a Drudger whose Session has finished",
			limit:    3,
			pool:     []*Drudger{busyDrudger(1), busyDrudger(2)},
			finished: []task.TaskID{busyTaskID(1)},
			wantSlot: 1,
		},
		{
			name:     "leaves a Drudger whose Session is still running",
			limit:    3,
			pool:     []*Drudger{busyDrudger(1), busyDrudger(2)},
			wantSlot: 3,
		},
		{
			name:    "fails when every slot is taken",
			limit:   2,
			pool:    []*Drudger{busyDrudger(1), busyDrudger(2)},
			wantErr: true,
		},
		{
			name:    "ignores slots above the limit but still fails when full",
			limit:   1,
			pool:    []*Drudger{busyDrudger(1), busyDrudger(7)},
			wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			for _, finishedTask := range testCase.finished {
				finishSession(t, workspace, finishedTask)
			}

			commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWithPool(
				&config.LocalConfig{ProjectSlug: testProjectSlug, MaxConcurrentDrudgers: testCase.limit},
				config.DefaultConfig(),
				commands,
				testCase.pool,
				taskToRun,
			)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })

			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected an error, the pool should have been full")
				}
				if !strings.Contains(err.Error(), config.MaxConcurrentDrudgersKey) {
					t.Errorf("expected the error to name the config key, got %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			claimed := service.drudgers.holderOf(taskToRun.ID)
			if claimed == nil {
				t.Fatalf("expected a Drudger to hold the task, got pool %v", service.drudgers.drudgers)
			}
			if claimed.Slot != testCase.wantSlot {
				t.Errorf("expected Drudger %d, got %d", testCase.wantSlot, claimed.Slot)
			}

			wantSandbox := testSandboxOfSlot(testCase.wantSlot)
			if create := commands.call(sbxCreateSubcommand); !slices.Contains(create, wantSandbox) {
				t.Errorf("expected the create call to name sandbox %q, got %v", wantSandbox, create)
			}
		})
	}
}

func TestDrudgerService_RunTask_WarnsAboutDrudgersAboveTheLimit(t *testing.T) {
	cases := []struct {
		name      string
		limit     int
		pool      []*Drudger
		wantNamed []int // slots the warning has to name, none means it stays quiet
		wantSlot  int
		wantErr   bool
	}{
		{
			name:     "a pool under the limit stays quiet",
			limit:    3,
			pool:     []*Drudger{idleDrudger(1)},
			wantSlot: 1,
		},
		{
			name:     "a pool at the limit stays quiet",
			limit:    2,
			pool:     []*Drudger{busyDrudger(1), idleDrudger(2)},
			wantSlot: 2,
		},
		{
			name:      "a lowered limit names the Drudgers above it and still allocates",
			limit:     3,
			pool:      []*Drudger{busyDrudger(1), busyDrudger(2), busyDrudger(4), busyDrudger(5)},
			wantNamed: []int{4, 5},
			wantSlot:  3,
		},
		{
			name:      "a full pool warns and still refuses a slot above the limit",
			limit:     2,
			pool:      []*Drudger{busyDrudger(1), busyDrudger(2), idleDrudger(3)},
			wantNamed: []int{3},
			wantErr:   true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith()}}
			service := newTestServiceWithPool(
				&config.LocalConfig{ProjectSlug: testProjectSlug, MaxConcurrentDrudgers: testCase.limit},
				config.DefaultConfig(),
				commands,
				testCase.pool,
				taskToRun,
			)

			var err error
			warnings := captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })

			if len(testCase.wantNamed) == 0 {
				if strings.Contains(warnings, config.MaxConcurrentDrudgersKey) {
					t.Errorf("expected no warning about the limit, got %q", warnings)
				}
			} else {
				if !strings.Contains(warnings, config.MaxConcurrentDrudgersKey) {
					t.Errorf("expected the warning to name the config key, got %q", warnings)
				}
				for _, entry := range testCase.pool {
					named := strings.Contains(warnings, entry.Sandbox)
					wanted := slices.Contains(testCase.wantNamed, entry.Slot)
					if named != wanted {
						t.Errorf("expected slot %d named in the warning: %t, got %t (warning %q)", entry.Slot, wanted, named, warnings)
					}
				}
			}

			if testCase.wantErr {
				if err == nil {
					t.Fatal("expected an error, the pool should have been full within the limit")
				}
				if holder := service.drudgers.holderOf(taskToRun.ID); holder != nil {
					t.Errorf("expected no Drudger to hold the task, got slot %d", holder.Slot)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			claimed := service.drudgers.holderOf(taskToRun.ID)
			if claimed == nil {
				t.Fatalf("expected a Drudger to hold the task, got pool %v", service.drudgers.drudgers)
			}
			if claimed.Slot != testCase.wantSlot {
				t.Errorf("expected Drudger %d, got %d", testCase.wantSlot, claimed.Slot)
			}
		})
	}
}

func TestDrudgerService_RunTask_LaunchesIntoTheStoredSandboxName(t *testing.T) {
	const namedByAnEarlierHarness = "drudge-opencode-test-project-1"

	workspace := setupWorkspace(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(namedByAnEarlierHarness)}}
	service := newTestServiceWithPool(
		&config.LocalConfig{ProjectSlug: testProjectSlug},
		config.DefaultConfig(),
		commands,
		[]*Drudger{{Slot: 1, Sandbox: namedByAnEarlierHarness}},
		taskToRun,
	)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := commands.subcommands(); !slices.Equal(got, []string{sbxLsSubcommand, sbxExecSubcommand}) {
		t.Errorf("expected the stored sandbox to be reused, got %v", got)
	}
	if start := commands.call(sbxExecSubcommand); !slices.Contains(start, namedByAnEarlierHarness) {
		t.Errorf("expected the start call to name sandbox %q, got %v", namedByAnEarlierHarness, start)
	}
	if claimed := service.drudgers.atSlot(1); claimed.Sandbox != namedByAnEarlierHarness {
		t.Errorf("expected the stored sandbox name to survive the run, got %q", claimed.Sandbox)
	}
}

// The agent works for as long as the task takes. Waiting for it would hold the
// terminal for the whole run, so only the launch is started without waiting,
// and every step the launch depends on is still waited for.
func TestDrudgerService_RunTask_LaunchesTheAgentWithoutWaitingForIt(t *testing.T) {
	workspace := setupWorkspace(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith()}}
	service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	wantAll := []string{sbxLsSubcommand, sbxCreateSubcommand, sbxExecSubcommand}
	if got := commands.subcommands(); !slices.Equal(got, wantAll) {
		t.Errorf("expected commands %v, got %v", wantAll, got)
	}

	wantStarted := []string{sbxExecSubcommand}
	if got := commands.startedSubcommands(); !slices.Equal(got, wantStarted) {
		t.Errorf("expected only %v to be started without waiting, got %v", wantStarted, got)
	}
}

func TestDrudgerService_RunTask_DryRunClaimsNothing(t *testing.T) {
	workspace := setupWorkspace(t)
	taskToRun := todoTask()
	commands := &fakeCommandRunner{workspace: workspace}
	service := newTestServiceWithPool(
		&config.LocalConfig{ProjectSlug: testProjectSlug},
		config.DefaultConfig(),
		commands,
		[]*Drudger{busyDrudger(1)},
		taskToRun,
	)

	var err error
	out := captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, true) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A dry run still says which Drudger the real run would take.
	if !strings.Contains(out, testSandboxSlot2) {
		t.Errorf("expected the dry run to name sandbox %q, got %q", testSandboxSlot2, out)
	}
	if len(service.drudgers.drudgers) != 1 {
		t.Errorf("expected the pool to be left alone, got %v", service.drudgers.drudgers)
	}
	if held := service.drudgers.holderOf(taskToRun.ID); held != nil {
		t.Errorf("expected no Drudger to hold the task, got Drudger %d", held.Slot)
	}
	if recorded := service.drudgers.atSlot(1); recorded.Health != HealthUnknown {
		t.Errorf("expected a dry run to record no health, got %q", recorded.Health)
	}
	if len(commands.calls) != 0 {
		t.Errorf("expected a dry run to run nothing, got %v", commands.subcommands())
	}
}

func TestDrudgerService_RunTask_UsesTheConfiguredPromptFile(t *testing.T) {
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

func TestDrudgerService_RunTask_PromptFileMissingPlaceholderNamesTheFile(t *testing.T) {
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

func TestDrudgerService_RunTask_RecordsWhatItSawOfTheSandbox(t *testing.T) {
	const otherRepo = "/some/other/repo"
	sbxErr := fmt.Errorf("sbx: no such binary")

	cases := []struct {
		name       string
		listing    string
		listErr    error
		createErr  error
		wantHealth Health
		wantErr    bool
	}{
		{
			name:       "the sandbox is there on the workspace of the run",
			listing:    sandboxListingWith(testSandbox),
			wantHealth: HealthUsable,
		},
		{
			name:       "the listing omits the sandbox, so it is built",
			listing:    sandboxListingWith(),
			wantHealth: HealthUsable,
		},
		{
			name:       "the sandbox is missing and cannot be built",
			listing:    sandboxListingWith(),
			createErr:  sbxErr,
			wantHealth: HealthGone,
			wantErr:    true,
		},
		{
			name:       "the sandbox is mounted on another repository",
			listing:    sandboxListingMountedOn(testSandbox, otherRepo),
			wantHealth: HealthMisplaced,
			wantErr:    true,
		},
		{
			name:       "the listing cannot be read, so nothing was learned",
			listErr:    sbxErr,
			wantHealth: HealthUnknown,
			wantErr:    true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{
				workspace: workspace,
				outputs:   []string{testCase.listing},
				errs:      []error{testCase.listErr, testCase.createErr},
			}
			service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			if testCase.wantErr != (err != nil) {
				t.Fatalf("expected an error %t, got %v", testCase.wantErr, err)
			}

			recorded := service.drudgers.atSlot(1)
			if recorded == nil {
				t.Fatalf("expected Drudger 1 to be in the pool, got %v", service.drudgers.drudgers)
			}
			if recorded.Health != testCase.wantHealth {
				t.Errorf("expected health %q, got %q", testCase.wantHealth, recorded.Health)
			}
			if recorded.LastChecked.IsZero() {
				t.Error("expected last checked to be stamped")
			}
		})
	}
}
