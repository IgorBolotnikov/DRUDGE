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

	// stoppedSandboxStatus is the sbx status of a sandbox with nothing running
	// in it.
	stoppedSandboxStatus = "stopped"
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
type fakeCommandRunner struct {
	workspace string
	calls     [][]string
	started   [][]string
	outputs   []string
	stderrs   []string
	errs      []error
	// onStart stands in for the agent, which writes to its run directory only
	// after it is started. A runner without one writes an init event, which is
	// what a real agent writes first.
	onStart func()
	// silentAgent stands for a start that forked and died, leaving an empty
	// run directory behind.
	silentAgent bool
}

func (runner *fakeCommandRunner) Start(argv []string) error {
	runner.started = append(runner.started, argv)
	_, _, err := runner.Run(argv)
	if err != nil {
		return err
	}
	switch {
	case runner.onStart != nil:
		runner.onStart()
	case !runner.silentAgent:
		writeInitEventOf(argv)
	}
	return nil
}

// writeInitEventOf writes an init event into the stream file that a launcher
// script redirects its agent to.
func writeInitEventOf(argv []string) {
	path := streamPathIn(argv)
	if path == "" {
		return
	}
	_ = os.WriteFile(path, []byte(initEvent+"\n"), common.DefaultFilePerm)
}

// streamPathIn reads the event stream path out of a launcher script. The
// script redirects its agent on the second line, and the stream is the first
// of the two redirects.
func streamPathIn(argv []string) string {
	lines := strings.Split(argv[len(argv)-1], "\n")
	if len(lines) < 2 {
		return ""
	}
	_, redirected, found := strings.Cut(lines[1], " > ")
	if !found {
		return ""
	}
	quoted, _, found := strings.Cut(redirected, " 2> ")
	if !found {
		return ""
	}
	return shellUnquote(quoted)
}

// shellUnquote turns a value the launcher quoted back into the path it names.
func shellUnquote(quoted string) string {
	bare := strings.TrimSuffix(strings.TrimPrefix(quoted, "'"), "'")
	return strings.ReplaceAll(bare, `'\''`, "'")
}

func (runner *fakeCommandRunner) Run(argv []string) (string, string, error) {
	index := len(runner.calls)
	runner.calls = append(runner.calls, argv)

	var output string
	if index < len(runner.outputs) {
		output = inWorkspace(runner.outputs[index], runner.workspace)
	}
	var stderr string
	if index < len(runner.stderrs) {
		stderr = runner.stderrs[index]
	}
	var err error
	if index < len(runner.errs) {
		err = runner.errs[index]
	}
	return output, stderr, err
}

func (runner *fakeCommandRunner) subcommands() []string {
	names := make([]string, 0, len(runner.calls))
	for _, argv := range runner.calls {
		names = append(names, argv[1])
	}
	return names
}

func (runner *fakeCommandRunner) startedSubcommands() []string {
	names := make([]string, 0, len(runner.started))
	for _, argv := range runner.started {
		names = append(names, argv[1])
	}
	return names
}

// callCount returns how many times an sbx subcommand was run.
func (runner *fakeCommandRunner) callCount(subcommand string) int {
	count := 0
	for _, argv := range runner.calls {
		if argv[1] == subcommand {
			count++
		}
	}
	return count
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
	// lockHeld stands for another drudge command holding the lock, which is
	// what a caller that would rather not wait runs into.
	lockHeld bool
	// updates counts the writes the repo was asked to make.
	updates int
}

func (repo *fakeDrudgerRepo) ListDrudgers(projectSlug string) ([]*Drudger, error) {
	return copyDrudgers(repo.drudgers), nil
}

func (repo *fakeDrudgerRepo) UpdateDrudgers(projectSlug string, change func([]*Drudger) ([]*Drudger, error)) error {
	updated, err := change(copyDrudgers(repo.drudgers))
	if err != nil {
		return err
	}
	repo.updates++
	repo.drudgers = updated
	return nil
}

func (repo *fakeDrudgerRepo) TryUpdateDrudgers(projectSlug string, change func([]*Drudger) ([]*Drudger, error)) (bool, error) {
	if repo.lockHeld {
		return false, nil
	}
	return true, repo.UpdateDrudgers(projectSlug, change)
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
	service := New(logger, localCfg, globalCfg, task.NewTaskService(&fakeTaskRepo{tasks: tasks}, logger), drudgers, commands)
	// Tests check what a retry and a grace period do. Sitting through the real
	// durations adds nothing.
	service.daemonRetryDelay = 0
	service.launchGrace = 0
	return &testService{
		DrudgerService: service,
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
	return sandboxListingIn(sbxStatusRunning, names...)
}

func sandboxListingStopped(names ...string) string {
	return sandboxListingIn(stoppedSandboxStatus, names...)
}

func sandboxListingIn(status string, names ...string) string {
	entries := make([]string, 0, len(names))
	for _, name := range names {
		entries = append(entries, sandboxEntry(name, status, runWorkspace))
	}
	return sandboxListingOf(entries...)
}

func sandboxListingMountedOn(name string, mounts ...string) string {
	return sandboxListingOf(sandboxEntry(name, sbxStatusRunning, mounts...))
}

func sandboxEntry(name string, status string, mounts ...string) string {
	quoted := make([]string, 0, len(mounts))
	for _, mount := range mounts {
		quoted = append(quoted, fmt.Sprintf("%q", mount))
	}
	return fmt.Sprintf(`{"name":%q,"status":%q,"workspaces":[%s]}`, name, status, strings.Join(quoted, ","))
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
	if taskToRun.SessionID != sampleSessionID {
		t.Errorf("expected session id %q, got %q", sampleSessionID, taskToRun.SessionID)
	}
}

func TestDrudgerService_RunTask_RecordsTheSessionIDTheAgentHasWritten(t *testing.T) {
	cases := []struct {
		name string
		// lines stand for what the agent has written to its stream by the time
		// the launch returns.
		lines []string
		want  string
	}{
		{name: "the agent is halfway through its first event", lines: []string{`{"type":"system","subty`}},
		{name: "the agent has written its init event", lines: []string{initEvent}, want: sampleSessionID},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()

			commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(testSandbox)}}
			if testCase.lines != nil {
				commands.onStart = func() {
					writeStream(t, common.RunDir(workspace, string(taskToRun.ID)), testCase.lines...)
				}
			}
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
				if recorded := service.drudgers.atSlot(1); recorded.SandboxHealth != SandboxMisplaced {
					t.Errorf("expected the refusal to be recorded as %q, got %q", SandboxMisplaced, recorded.SandboxHealth)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if recorded := service.drudgers.atSlot(1); recorded.SandboxHealth != SandboxUsable {
				t.Errorf("expected the sandbox to be recorded as %q, got %q", SandboxUsable, recorded.SandboxHealth)
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
		name    string
		outputs []string
		errs    []error
		// silentAgent stands for a launch that forked and died without
		// starting an agent.
		silentAgent bool
		wantRuns    int
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
			name:     "starting the agent fails",
			outputs:  []string{sandboxListingWith(testSandbox)},
			errs:     []error{nil, spawnErr},
			wantRuns: 2,
		},
		{
			name:        "the agent never comes up",
			outputs:     []string{sandboxListingWith(testSandbox)},
			silentAgent: true,
			wantRuns:    2,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{
				workspace:   workspace,
				outputs:     testCase.outputs,
				errs:        testCase.errs,
				silentAgent: testCase.silentAgent,
			}
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

			// A launch creates the run directory before the sandbox steps, so
			// it is there whatever fails afterwards.
			exists, err := common.Exists(common.RunDir(workspace, string(taskToRun.ID)))
			if err != nil {
				t.Fatalf("could not check the run directory: %v", err)
			}
			if !exists {
				t.Error("expected the run directory of the claimed slot to be there")
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
	if recorded := service.drudgers.atSlot(1); recorded.SandboxHealth != SandboxUnchecked {
		t.Errorf("expected a dry run to record no sandbox health, got %q", recorded.SandboxHealth)
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
		wantHealth SandboxHealth
		wantErr    bool
	}{
		{
			name:       "the sandbox is there on the workspace of the run",
			listing:    sandboxListingWith(testSandbox),
			wantHealth: SandboxUsable,
		},
		{
			name:       "the listing omits the sandbox, so it is built",
			listing:    sandboxListingWith(),
			wantHealth: SandboxUsable,
		},
		{
			name:       "the sandbox is missing and cannot be built",
			listing:    sandboxListingWith(),
			createErr:  sbxErr,
			wantHealth: SandboxGone,
			wantErr:    true,
		},
		{
			name:       "the sandbox is mounted on another repository",
			listing:    sandboxListingMountedOn(testSandbox, otherRepo),
			wantHealth: SandboxMisplaced,
			wantErr:    true,
		},
		{
			name:       "the listing cannot be read, so nothing was learned",
			listErr:    sbxErr,
			wantHealth: SandboxUnchecked,
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
			if recorded.SandboxHealth != testCase.wantHealth {
				t.Errorf("expected sandbox health %q, got %q", testCase.wantHealth, recorded.SandboxHealth)
			}
			if recorded.LastChecked.IsZero() {
				t.Error("expected last checked to be stamped")
			}
		})
	}
}

func TestDrudgerService_RunTask_CopesWithTheSbxDaemon(t *testing.T) {
	// What sbx really writes to stderr in each of these situations.
	const (
		coldStartStderr  = "Starting sandboxd daemon..."
		daemonDownStderr = "ERROR: ensure daemon: daemon exited unexpectedly: exit status 1"
		noBinaryStderr   = "sbx: no such binary"
	)

	daemonDown := fmt.Errorf("command sbx failed: exit status 1: %s", daemonDownStderr)
	noBinary := fmt.Errorf("command sbx failed: %s", noBinaryStderr)

	cases := []struct {
		name            string
		outputs         []string
		stderrs         []string
		errs            []error
		wantListings    int
		wantLogContains string
		wantErrContains string
	}{
		{
			name:            "a cold start is reported and the run goes on",
			outputs:         []string{sandboxListingWith(testSandbox)},
			stderrs:         []string{coldStartStderr},
			wantListings:    1,
			wantLogContains: "daemon was not running",
		},
		{
			name:            "a daemon that comes up on the second try costs one retry",
			outputs:         []string{"", sandboxListingWith(testSandbox)},
			stderrs:         []string{daemonDownStderr},
			errs:            []error{daemonDown},
			wantListings:    2,
			wantLogContains: "one more try",
		},
		{
			name:            "a daemon that never comes up names the daemon",
			outputs:         []string{"", ""},
			stderrs:         []string{daemonDownStderr, daemonDownStderr},
			errs:            []error{daemonDown, daemonDown},
			wantListings:    2,
			wantErrContains: "sbx daemon status",
		},
		{
			name:            "any other listing failure is not retried",
			stderrs:         []string{noBinaryStderr},
			errs:            []error{noBinary},
			wantListings:    1,
			wantErrContains: "could not list the sandboxes",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()
			commands := &fakeCommandRunner{
				workspace: workspace,
				outputs:   testCase.outputs,
				stderrs:   testCase.stderrs,
				errs:      testCase.errs,
			}
			service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

			var err error
			logged := captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })

			if got := commands.callCount(sbxLsSubcommand); got != testCase.wantListings {
				t.Errorf("expected %d listings, got %d in %v", testCase.wantListings, got, commands.subcommands())
			}
			if testCase.wantLogContains != "" && !strings.Contains(logged, testCase.wantLogContains) {
				t.Errorf("expected the log to say %q, got %q", testCase.wantLogContains, logged)
			}

			if testCase.wantErrContains != "" {
				if err == nil {
					t.Fatal("expected the failure to surface")
				}
				if !strings.Contains(err.Error(), testCase.wantErrContains) {
					t.Errorf("expected error to say %q, got %q", testCase.wantErrContains, err)
				}
				if taskToRun.Status != task.StatusTodo {
					t.Errorf("expected the task to stay %q, got %q", task.StatusTodo, taskToRun.Status)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if taskToRun.Status != task.StatusInProgress {
				t.Errorf("expected the task to be %q, got %q", task.StatusInProgress, taskToRun.Status)
			}
		})
	}
}

func TestDrudgerService_RunTask_ClearsWhatThePreviousRunLeft(t *testing.T) {
	workspace := setupWorkspace(t)

	// A task the vendor refused, which put it back in todo and left the whole
	// finished run behind.
	taskToRun := todoTask()
	taskToRun.VendorError = authRefusedText
	taskToRun.VendorErrorClass = task.VendorErrorAuth

	runDir := common.RunDir(workspace, string(taskToRun.ID))
	writeStream(t, runDir, initEvent, authRefusedEvent, authRefusedResultEvent)
	writeExit(t, runDir, "1\n")

	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(testSandbox)}}
	service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	finished, err := sessionFinished(runDir)
	if err != nil {
		t.Fatalf("could not check the run directory: %v", err)
	}
	if finished {
		t.Error("expected the exit code of the previous run to be gone")
	}

	report, err := readSessionReport(runDir, time.Now().UTC())
	if err != nil {
		t.Fatalf("could not read the run directory: %v", err)
	}
	if report.Status != StatusWorking {
		t.Errorf("expected the new run to read as %q, got %q", StatusWorking, report.Status)
	}
	if report.Result != nil {
		t.Errorf("expected the terminal event of the previous run to be gone, got %+v", report.Result)
	}
	if taskToRun.VendorError != "" || taskToRun.VendorErrorClass != "" {
		t.Errorf("expected the previous refusal to be cleared, got %q as %q", taskToRun.VendorError, taskToRun.VendorErrorClass)
	}
}

func TestDrudgerService_RerunTask_OnlyRerunsTasksAnAgentHasHad(t *testing.T) {
	cases := []struct {
		name   string
		status task.TaskStatus
		// finishedRun says whether the task has a run directory holding a run
		// that is over, which is what an in-progress task needs to start again.
		finishedRun bool
		wantErr     bool
	}{
		{name: "reruns an in-progress task whose Session is over", status: task.StatusInProgress, finishedRun: true},
		{name: "reruns a fucked-up task", status: task.StatusFuckedUp},
		{name: "refuses a draft task", status: task.StatusDraft, wantErr: true},
		{name: "refuses a todo task", status: task.StatusTodo, wantErr: true},
		{name: "refuses a done task", status: task.StatusDone, wantErr: true},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRerun := todoTask()
			taskToRerun.Status = testCase.status
			if testCase.finishedRun {
				finishSession(t, workspace, taskToRerun.ID)
			}

			commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(testSandbox)}}
			service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRerun)

			var err error
			out := captureOutput(func() { err = service.RerunTask(testProjectSlug, taskToRerun.ID, false) })

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error for status %q", testCase.status)
				}
				if !strings.Contains(err.Error(), string(testCase.status)) {
					t.Errorf("expected the error to name status %q, got %q", testCase.status, err)
				}
				if len(commands.calls) != 0 {
					t.Errorf("expected a refused rerun to run nothing, got %v", commands.subcommands())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if taskToRerun.Status != task.StatusInProgress {
				t.Errorf("expected status %q, got %q", task.StatusInProgress, taskToRerun.Status)
			}
			if taskToRerun.StartedAt.IsZero() {
				t.Error("expected started at to be stamped")
			}
			if !strings.Contains(out, string(testCase.status)) {
				t.Errorf("expected the rerun to say the task was %q, got %q", testCase.status, out)
			}
		})
	}
}

func TestDrudgerService_RerunTask_RefusesATaskWhoseAgentIsStillWorking(t *testing.T) {
	workspace := setupWorkspace(t)
	taskToRerun := todoTask()
	taskToRerun.Status = task.StatusInProgress

	// A stream with no exit file beside it is an agent that is still writing.
	runDir := common.RunDir(workspace, string(taskToRerun.ID))
	writeStream(t, runDir, initEvent)

	working := idleDrudger(1)
	working.TaskID = taskToRerun.ID

	commands := &fakeCommandRunner{workspace: workspace}
	service := newTestServiceWithPool(
		&config.LocalConfig{ProjectSlug: testProjectSlug},
		config.DefaultConfig(),
		commands,
		[]*Drudger{working},
		taskToRerun,
	)

	var err error
	captureOutput(func() { err = service.RerunTask(testProjectSlug, taskToRerun.ID, false) })
	if err == nil {
		t.Fatal("expected a rerun of a task with a working agent to be refused")
	}
	for _, want := range []string{testSandbox, string(taskToRerun.ID), nukeCommand} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("expected the error to name %q, got %q", want, err)
		}
	}

	if len(commands.calls) != 0 {
		t.Errorf("expected a refused rerun to launch nothing, got %v", commands.subcommands())
	}
	if !taskToRerun.StartedAt.IsZero() {
		t.Error("expected the task record to be left alone")
	}

	report, err := readSessionReport(runDir, time.Now().UTC())
	if err != nil {
		t.Fatalf("could not read the run directory: %v", err)
	}
	if report.Status != StatusWorking {
		t.Errorf("expected the run directory to be left alone, got %q", report.Status)
	}
}

func TestDrudgerService_RerunTask_TakesATaskWhoseSlotWasReclaimed(t *testing.T) {
	workspace := setupWorkspace(t)
	taskToRerun := todoTask()
	taskToRerun.Status = task.StatusInProgress

	// The agent died without writing an exit file, so its run directory reads
	// as unfinished. A reclaim has already cleared its slot, and that idle
	// slot is what says the agent is gone.
	runDir := common.RunDir(workspace, string(taskToRerun.ID))
	writeStream(t, runDir, initEvent)

	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(testSandbox)}}
	service := newTestServiceWithPool(
		&config.LocalConfig{ProjectSlug: testProjectSlug},
		config.DefaultConfig(),
		commands,
		[]*Drudger{idleDrudger(1)},
		taskToRerun,
	)

	var err error
	captureOutput(func() { err = service.RerunTask(testProjectSlug, taskToRerun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if taskToRerun.Status != task.StatusInProgress {
		t.Errorf("expected the task to be handed to an agent again, got %q", taskToRerun.Status)
	}
	if held := service.drudgers.holderOf(taskToRerun.ID); held == nil {
		t.Errorf("expected a Drudger to hold the task again, got pool %v", service.drudgers.drudgers)
	}
}

func TestDrudgerService_RerunTask_ClearsTheFinishedRun(t *testing.T) {
	workspace := setupWorkspace(t)
	taskToRerun := todoTask()
	taskToRerun.Status = task.StatusFuckedUp
	taskToRerun.SessionResult = "gave up"
	taskToRerun.SessionTurns = 12
	taskToRerun.FinishedAt = time.Now().UTC()

	runDir := common.RunDir(workspace, string(taskToRerun.ID))
	writeStream(t, runDir, initEvent, resultEvent)
	writeExit(t, runDir, "1\n")

	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(testSandbox)}}
	service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRerun)

	var err error
	captureOutput(func() { err = service.RerunTask(testProjectSlug, taskToRerun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	report, err := readSessionReport(runDir, time.Now().UTC())
	if err != nil {
		t.Fatalf("could not read the run directory: %v", err)
	}
	if report.Status != StatusWorking {
		t.Errorf("expected the new run to read as %q, got %q", StatusWorking, report.Status)
	}
	if report.Result != nil {
		t.Errorf("expected the terminal event of the previous run to be gone, got %+v", report.Result)
	}

	if !taskToRerun.FinishedAt.IsZero() {
		t.Error("expected the finish time of the previous run to be cleared")
	}
	if taskToRerun.SessionResult != "" || taskToRerun.SessionTurns != 0 {
		t.Errorf("expected the previous outcome to be cleared, got %q in %d turns", taskToRerun.SessionResult, taskToRerun.SessionTurns)
	}
}

func TestDrudgerService_RerunTask_LaunchesTheSameWayARunDoes(t *testing.T) {
	workspace := setupWorkspace(t)
	taskToRun := todoTask()

	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingWith(testSandbox)}}
	service := newTestServiceWith(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, taskToRun)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error on the first run: %v", err)
	}
	firstRun := slices.Clone(commands.calls)

	finishSession(t, workspace, taskToRun.ID)
	taskToRun.Status = task.StatusFuckedUp
	commands.calls = nil
	commands.started = nil

	captureOutput(func() { err = service.RerunTask(testProjectSlug, taskToRun.ID, false) })
	if err != nil {
		t.Fatalf("unexpected error on the rerun: %v", err)
	}

	if !slices.EqualFunc(firstRun, commands.calls, slices.Equal) {
		t.Errorf("expected a rerun to make the same sbx calls as a run, got %v after %v", commands.calls, firstRun)
	}
}

func TestDrudgerService_RerunTask_DryRunLeavesThePreviousRunAlone(t *testing.T) {
	workspace := setupWorkspace(t)
	taskToRerun := todoTask()
	taskToRerun.Status = task.StatusFuckedUp

	runDir := common.RunDir(workspace, string(taskToRerun.ID))
	writeStream(t, runDir, initEvent, resultEvent)
	writeExit(t, runDir, "0\n")

	commands := &fakeCommandRunner{workspace: workspace}
	service := newTestServiceWithPool(
		&config.LocalConfig{ProjectSlug: testProjectSlug},
		config.DefaultConfig(),
		commands,
		[]*Drudger{busyDrudger(1)},
		taskToRerun,
	)

	var err error
	out := captureOutput(func() { err = service.RerunTask(testProjectSlug, taskToRerun.ID, true) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// A dry run still says which Drudger the real rerun would take.
	if !strings.Contains(out, testSandboxSlot2) {
		t.Errorf("expected the dry run to name sandbox %q, got %q", testSandboxSlot2, out)
	}
	if len(commands.calls) != 0 {
		t.Errorf("expected a dry run to run nothing, got %v", commands.subcommands())
	}
	if held := service.drudgers.holderOf(taskToRerun.ID); held != nil {
		t.Errorf("expected no Drudger to hold the task, got Drudger %d", held.Slot)
	}
	if taskToRerun.Status != task.StatusFuckedUp {
		t.Errorf("expected the task to stay %q, got %q", task.StatusFuckedUp, taskToRerun.Status)
	}

	report, err := readSessionReport(runDir, time.Now().UTC())
	if err != nil {
		t.Fatalf("could not read the run directory: %v", err)
	}
	if report.Status != StatusGotShitDone {
		t.Errorf("expected the previous run to be left alone, got %q", report.Status)
	}
}

func TestDrudgerService_RecordsBothPartsOfADrudger(t *testing.T) {
	sbxErr := fmt.Errorf("sbx: no such binary")

	cases := []struct {
		name string
		// pool is what the Drudgers file already holds, which is where a part
		// observed by an earlier run comes from.
		pool []*Drudger
		// listing is what the sandbox listing says, and createErr fails the
		// build of a sandbox the listing omits.
		listing   string
		createErr error
		// stream is what the agent writes once it is started, and exit is what
		// it exits with. An empty stream stands for a launch that never got as
		// far as an agent.
		stream []string
		exit   string

		wantSandbox SandboxHealth
		wantAgent   AgentHealth
	}{
		{
			name:        "a clean run leaves both parts fine",
			listing:     sandboxListingWith(testSandbox),
			stream:      []string{initEvent, resultEvent},
			exit:        "0\n",
			wantSandbox: SandboxUsable,
			wantAgent:   AgentReady,
		},
		{
			name:        "a run the vendor refused breaks the agent part alone",
			listing:     sandboxListingWith(testSandbox),
			stream:      []string{initEvent, authRefusedEvent, authRefusedResultEvent},
			exit:        "1\n",
			wantSandbox: SandboxUsable,
			wantAgent:   AgentRefused,
		},
		{
			name:        "an agent that failed the work on its own is still fine",
			listing:     sandboxListingWith(testSandbox),
			stream:      []string{initEvent, erroredResultEvent},
			exit:        "1\n",
			wantSandbox: SandboxUsable,
			wantAgent:   AgentReady,
		},
		{
			name:        "a sandbox that cannot be built leaves the agent part alone",
			listing:     sandboxListingWith(),
			createErr:   sbxErr,
			wantSandbox: SandboxGone,
			wantAgent:   AgentUnchecked,
		},
		{
			name:        "a sandbox gone under a refused agent reads as both",
			pool:        []*Drudger{{Slot: 1, Sandbox: testSandbox, SandboxHealth: SandboxUsable, AgentHealth: AgentRefused}},
			listing:     sandboxListingWith(),
			createErr:   sbxErr,
			wantSandbox: SandboxGone,
			wantAgent:   AgentRefused,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)
			taskToRun := todoTask()

			commands := &fakeCommandRunner{
				workspace: workspace,
				outputs:   []string{testCase.listing},
				errs:      []error{nil, testCase.createErr},
			}
			if len(testCase.stream) > 0 {
				commands.onStart = func() {
					runDir := common.RunDir(workspace, string(taskToRun.ID))
					writeStream(t, runDir, testCase.stream...)
					writeExit(t, runDir, testCase.exit)
				}
			}
			service := newTestServiceWithPool(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, testCase.pool, taskToRun)

			captureOutput(func() {
				// A launch that fails says so, and the run it never made is the
				// point of the case that does it.
				if err := service.RunTask(testProjectSlug, taskToRun.ID, false); err != nil {
					return
				}
				if _, err := service.SessionStatus(testProjectSlug, taskToRun.ID); err != nil {
					t.Errorf("could not report on the Session: %v", err)
				}
			})

			recorded := service.drudgers.atSlot(1)
			if recorded == nil {
				t.Fatalf("expected Drudger 1 to be in the pool, got %v", service.drudgers.drudgers)
			}
			if recorded.SandboxHealth != testCase.wantSandbox {
				t.Errorf("expected sandbox health %q, got %q", testCase.wantSandbox, recorded.SandboxHealth)
			}
			if recorded.AgentHealth != testCase.wantAgent {
				t.Errorf("expected agent health %q, got %q", testCase.wantAgent, recorded.AgentHealth)
			}
			if recorded.LastChecked.IsZero() {
				t.Error("expected last checked to be stamped")
			}
		})
	}
}

func TestDrudgerService_ListDrudgers_ReclaimsFinishedSessions(t *testing.T) {
	claimedAt := time.Now().UTC().Add(-time.Hour)

	cases := []struct {
		name string
		// stream is what the agent of the claimed task wrote, and exit is what
		// it exited with. noExitFile stands for a Session that is still going.
		stream []string
		exit   string
		// noRunDir stands for a claim whose run directory was never created.
		noRunDir bool
		// lockHeld stands for another drudge command holding the Drudgers lock.
		lockHeld bool

		// wantTask is the task the Drudger of slot 1 still holds, empty when
		// the slot was freed.
		wantTask  task.TaskID
		wantAgent AgentHealth
		// wantLooked says whether the record was stamped as re-examined.
		wantLooked bool
	}{
		{
			name:       "a Session that is over frees the slot",
			stream:     []string{initEvent, resultEvent},
			exit:       "0\n",
			wantAgent:  AgentReady,
			wantLooked: true,
		},
		{
			name:       "a Session the vendor refused frees the slot and names the agent",
			stream:     []string{initEvent, authRefusedEvent, authRefusedResultEvent},
			exit:       "1\n",
			wantAgent:  AgentRefused,
			wantLooked: true,
		},
		{
			name:      "a Session that is still going keeps the slot",
			stream:    []string{initEvent, assistantEvent},
			exit:      noExitFile,
			wantTask:  busyTaskID(1),
			wantAgent: AgentUnchecked,
		},
		{
			name:      "a claim with no run directory is left alone",
			noRunDir:  true,
			wantTask:  busyTaskID(1),
			wantAgent: AgentUnchecked,
		},
		{
			name:      "a list that cannot take the lock reports what it read",
			stream:    []string{initEvent, resultEvent},
			exit:      "0\n",
			lockHeld:  true,
			wantTask:  busyTaskID(1),
			wantAgent: AgentUnchecked,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)

			claimed := busyDrudger(1)
			claimed.SandboxHealth = SandboxUsable
			claimed.LastChecked = claimedAt

			if !testCase.noRunDir {
				runDir := common.RunDir(workspace, string(claimed.TaskID))
				writeStream(t, runDir, testCase.stream...)
				if testCase.exit != noExitFile {
					writeExit(t, runDir, testCase.exit)
				}
			}

			pool := []*Drudger{idleDrudger(2), claimed}
			commands := &fakeCommandRunner{}
			service := newTestServiceWithPool(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, pool)
			service.drudgers.lockHeld = testCase.lockHeld

			var listed []*Drudger
			var err error
			captureOutput(func() { listed, err = service.ListDrudgers(testProjectSlug) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// A list reads run directories and runs no commands, which keeps
			// it fast.
			if len(commands.calls) != 0 {
				t.Errorf("expected a list to run no commands, got %v", commands.subcommands())
			}

			if len(listed) != 2 {
				t.Fatalf("expected both Drudgers to be listed, got %v", listed)
			}
			reported := listed[0]
			if reported.Slot != 1 {
				t.Fatalf("expected the lowest slot first, got slot %d", reported.Slot)
			}

			if reported.TaskID != testCase.wantTask {
				t.Errorf("expected the list to report task %q, got %q", testCase.wantTask, reported.TaskID)
			}
			if reported.AgentHealth != testCase.wantAgent {
				t.Errorf("expected the list to report agent health %q, got %q", testCase.wantAgent, reported.AgentHealth)
			}
			// A list looks at run directories alone, so what it says about a
			// sandbox is whatever the last launch saw.
			if reported.SandboxHealth != SandboxUsable {
				t.Errorf("expected sandbox health %q to be left alone, got %q", SandboxUsable, reported.SandboxHealth)
			}

			// The record is what the next command reads, so it has to say the
			// same thing the list did.
			recorded := service.drudgers.atSlot(1)
			if recorded == nil {
				t.Fatalf("expected Drudger 1 to stay in the pool, got %v", service.drudgers.drudgers)
			}
			if recorded.TaskID != testCase.wantTask {
				t.Errorf("expected the record to hold task %q, got %q", testCase.wantTask, recorded.TaskID)
			}
			if recorded.AgentHealth != testCase.wantAgent {
				t.Errorf("expected the record to hold agent health %q, got %q", testCase.wantAgent, recorded.AgentHealth)
			}

			// A record that was not re-examined keeps the timestamp it had,
			// because restamping it would claim a look that never happened.
			looked := recorded.LastChecked.After(claimedAt)
			if looked != testCase.wantLooked {
				t.Errorf("expected re-examined to be %v, got last checked %s against a claim at %s", testCase.wantLooked, recorded.LastChecked, claimedAt)
			}

			idle := service.drudgers.atSlot(2)
			if !idle.LastChecked.IsZero() {
				t.Errorf("expected an idle Drudger to be left alone, got last checked %s", idle.LastChecked)
			}
		})
	}
}

func TestDrudgerService_ReclaimDrudgers(t *testing.T) {
	cases := []struct {
		name string
		// stream is what the agent of the claimed task wrote, and exit is what
		// it exited with. noExitFile stands for a Session that has not ended.
		stream []string
		exit   string
		// noRunDir stands for a claim whose run directory was never created.
		noRunDir bool
		// heldFor is how long the slot has been claimed. It defaults to an
		// hour, which is well past the grace period.
		heldFor time.Duration
		// listing is what the sandbox listing says.
		listing string

		// wantReason is the condition that freed the slot, empty when nothing
		// was freed.
		wantReason string
		// wantHolds says whether the Drudger still holds its task afterwards.
		wantHolds bool
		wantAgent AgentHealth
	}{
		{
			name:      "an agent that is working keeps its slot",
			stream:    []string{initEvent, assistantEvent},
			exit:      noExitFile,
			listing:   sandboxListingWith(testSandbox),
			wantHolds: true,
			wantAgent: AgentUnchecked,
		},
		{
			name:      "an agent that has gone quiet keeps its slot",
			stream:    []string{initEvent},
			exit:      noExitFile,
			listing:   sandboxListingWith(testSandbox),
			wantHolds: true,
			wantAgent: AgentUnchecked,
		},
		{
			name:       "an agent whose sandbox stopped loses its slot",
			stream:     []string{initEvent, assistantEvent},
			exit:       noExitFile,
			listing:    sandboxListingStopped(testSandbox),
			wantReason: stuckSandboxNotRunning,
			wantAgent:  AgentUnchecked,
		},
		{
			name:       "an agent whose sandbox is gone loses its slot",
			stream:     []string{initEvent, assistantEvent},
			exit:       noExitFile,
			listing:    sandboxListingWith(),
			wantReason: stuckSandboxNotRunning,
			wantAgent:  AgentUnchecked,
		},
		{
			name:      "a launch still building its sandbox keeps its slot",
			exit:      noExitFile,
			listing:   sandboxListingWith(),
			wantHolds: true,
			wantAgent: AgentUnchecked,
		},
		{
			name:       "a claim with no run directory loses its slot",
			noRunDir:   true,
			listing:    sandboxListingWith(testSandbox),
			wantReason: stuckNoRunDirectory,
			wantAgent:  AgentUnchecked,
		},
		{
			name:      "a claim with no run directory yet keeps its slot",
			noRunDir:  true,
			heldFor:   time.Second,
			listing:   sandboxListingWith(testSandbox),
			wantHolds: true,
			wantAgent: AgentUnchecked,
		},
		{
			name:      "a Session that has ended frees its slot the ordinary way",
			stream:    []string{initEvent, resultEvent},
			exit:      "0\n",
			listing:   sandboxListingWith(testSandbox),
			wantAgent: AgentReady,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			workspace := setupWorkspace(t)

			heldFor := testCase.heldFor
			if heldFor == 0 {
				heldFor = time.Hour
			}
			claimedAt := time.Now().UTC().Add(-heldFor)

			claimed := busyDrudger(1)
			claimed.SandboxHealth = SandboxUsable
			claimed.LastChecked = claimedAt

			if !testCase.noRunDir {
				runDir := common.RunDir(workspace, string(claimed.TaskID))
				writeStream(t, runDir, testCase.stream...)
				if testCase.exit != noExitFile {
					writeExit(t, runDir, testCase.exit)
				}
			}

			commands := &fakeCommandRunner{workspace: workspace, outputs: []string{testCase.listing}}
			pool := []*Drudger{claimed, idleDrudger(2)}
			service := newTestServiceWithPool(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, pool)

			var freed []FreedSlot
			var err error
			captureOutput(func() { freed, err = service.ReclaimDrudgers(testProjectSlug) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			// A Session with an exit file is freed by reclaimFinished and
			// stays out of the report.
			if testCase.wantReason == "" {
				if len(freed) != 0 {
					t.Fatalf("expected nothing to be reported as taken back, got %v", freed)
				}
			} else {
				if len(freed) != 1 {
					t.Fatalf("expected one slot to be reported as taken back, got %v", freed)
				}
				if freed[0].Slot != 1 || freed[0].TaskID != busyTaskID(1) || freed[0].Sandbox != testSandbox {
					t.Errorf("expected the report to name Drudger 1 and its task, got %+v", freed[0])
				}
				if freed[0].Reason != testCase.wantReason {
					t.Errorf("expected reason %q, got %q", testCase.wantReason, freed[0].Reason)
				}
			}

			var wantTask task.TaskID
			if testCase.wantHolds {
				wantTask = busyTaskID(1)
			}

			recorded := service.drudgers.atSlot(1)
			if recorded.TaskID != wantTask {
				t.Errorf("expected the record to hold task %q, got %q", wantTask, recorded.TaskID)
			}
			if recorded.AgentHealth != testCase.wantAgent {
				t.Errorf("expected agent health %q, got %q", testCase.wantAgent, recorded.AgentHealth)
			}
			// A reclaim writes TaskID and LastChecked, and leaves every other
			// field alone.
			if recorded.SandboxHealth != SandboxUsable {
				t.Errorf("expected sandbox health %q to be left alone, got %q", SandboxUsable, recorded.SandboxHealth)
			}

			idle := service.drudgers.atSlot(2)
			if !idle.Idle() || !idle.LastChecked.IsZero() {
				t.Errorf("expected an idle Drudger to be left alone, got %+v", idle)
			}
		})
	}
}

func TestDrudgerService_ReclaimDrudgers_LeavesTheTaskAlone(t *testing.T) {
	workspace := setupWorkspace(t)

	held := todoTask()
	held.Status = task.StatusInProgress
	claimed := busyDrudger(1)
	claimed.TaskID = held.ID
	claimed.LastChecked = time.Now().UTC().Add(-time.Hour)

	runDir := common.RunDir(workspace, string(held.ID))
	writeStream(t, runDir, initEvent, assistantEvent)

	commands := &fakeCommandRunner{workspace: workspace, outputs: []string{sandboxListingStopped(testSandbox)}}
	service := newTestServiceWithPool(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, []*Drudger{claimed}, held)

	var freed []FreedSlot
	var err error
	captureOutput(func() { freed, err = service.ReclaimDrudgers(testProjectSlug) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(freed) != 1 {
		t.Fatalf("expected the stuck slot to be taken back, got %v", freed)
	}

	if held.Status != task.StatusInProgress {
		t.Errorf("expected the task to be left %q, got %q", task.StatusInProgress, held.Status)
	}
	if !held.FinishedAt.IsZero() {
		t.Error("expected the task to be left without a finish time")
	}

	// The run directory holds what the dead agent wrote, and a reclaim does
	// not touch it.
	present, err := common.Exists(runDir)
	if err != nil {
		t.Fatalf("could not check the run directory: %v", err)
	}
	if !present {
		t.Error("expected the run directory to be left where it is")
	}
}

func TestDrudgerService_ReclaimDrudgers_RefusesToGuessWithoutAListing(t *testing.T) {
	setupWorkspace(t)

	claimed := busyDrudger(1)
	claimed.LastChecked = time.Now().UTC().Add(-time.Hour)
	commands := &fakeCommandRunner{errs: []error{fmt.Errorf("sbx: no such binary")}}
	service := newTestServiceWithPool(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), commands, []*Drudger{claimed})

	var err error
	captureOutput(func() { _, err = service.ReclaimDrudgers(testProjectSlug) })
	if err == nil {
		t.Fatal("expected a listing that failed to stop the reclaim")
	}
	if !strings.Contains(err.Error(), "could not list the sandboxes") {
		t.Errorf("expected the error to name the listing, got %q", err)
	}

	if service.drudgers.atSlot(1).Idle() {
		t.Error("expected the claim to be left alone when nothing could be checked")
	}
}

func TestDrudgerService_ListDrudgers_WritesNothingWithNoSlotToFree(t *testing.T) {
	setupWorkspace(t)

	pool := []*Drudger{idleDrudger(1), idleDrudger(2)}
	service := newTestServiceWithPool(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), &fakeCommandRunner{}, pool)

	var err error
	captureOutput(func() { _, err = service.ListDrudgers(testProjectSlug) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if service.drudgers.updates != 0 {
		t.Errorf("expected a list of idle Drudgers to write nothing, got %d writes", service.drudgers.updates)
	}
}
