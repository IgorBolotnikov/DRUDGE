package runner

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"drudge/internal/common"
	"drudge/internal/config"
	"drudge/internal/task"
)

const testProjectSlug = "test-project"

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

type fakeCommandRunner struct {
	argv   []string
	output string
	err    error
}

func (runner *fakeCommandRunner) Run(argv []string) (string, error) {
	runner.argv = argv
	return runner.output, runner.err
}

func newTestService(tasks ...*task.Task) *RunnerService {
	return newTestServiceWithConfigs(&config.LocalConfig{ProjectSlug: testProjectSlug}, config.DefaultConfig(), tasks...)
}

func newTestServiceWithConfigs(localCfg *config.LocalConfig, globalCfg *config.GlobalConfig, tasks ...*task.Task) *RunnerService {
	logger := common.NewLogger("")
	return New(logger, localCfg, globalCfg, task.NewTaskService(&fakeTaskRepo{tasks: tasks}, logger), &fakeCommandRunner{})
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

func TestRunnerService_RunTask_DryRunStatusGuard(t *testing.T) {
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
			service := newTestService(&task.Task{
				ID:          "task-1",
				Title:       "Fix login",
				Description: "SSO is broken",
				Status:      testCase.status,
				ProjectSlug: testProjectSlug,
			})

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, "task-1", true) })

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

func TestRunnerService_RunTask_DryRunPrintsPrompt(t *testing.T) {
	taskToRun := &task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		TicketID:    "PROJ-123",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	}
	service := newTestService(taskToRun)

	var err error
	out := captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, true) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{taskToRun.Title, taskToRun.Description, taskToRun.TicketID} {
		if !strings.Contains(out, want) {
			t.Errorf("expected dry run output to contain %q, got %q", want, out)
		}
	}
}

func TestRunnerService_RunTask_UnknownTask(t *testing.T) {
	service := newTestService()

	err := service.RunTask(testProjectSlug, "nope", true)
	if err == nil {
		t.Fatal("expected an error for an unknown task ID")
	}
}

func TestRunnerService_RunTask_SpawnsAndRecordsTheRunner(t *testing.T) {
	cases := []struct {
		name          string
		output        string
		wantSessionID string
	}{
		{name: "single line of output", output: "sess-abc123", wantSessionID: "sess-abc123"},
		{name: "session on the last line", output: "starting sandbox\nsess-abc123", wantSessionID: "sess-abc123"},
		{name: "trailing blank lines are skipped", output: "sess-abc123\n\n  \n", wantSessionID: "sess-abc123"},
		{name: "no session reported", output: "  \n\n"},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			taskToRun := &task.Task{
				ID:          "task-1",
				Title:       "Fix login",
				Description: "SSO is broken",
				Status:      task.StatusTodo,
				ProjectSlug: testProjectSlug,
			}
			logger := common.NewLogger("")
			globalCfg := config.DefaultConfig()
			commands := &fakeCommandRunner{output: testCase.output}
			service := New(
				logger,
				&config.LocalConfig{ProjectSlug: testProjectSlug},
				globalCfg,
				task.NewTaskService(&fakeTaskRepo{tasks: []*task.Task{taskToRun}}, logger),
				commands,
			)

			var err error
			captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if commands.argv == nil {
				t.Fatal("expected the runner command to be run")
			}
			if commands.argv[0] != sbxBinary {
				t.Errorf("expected the argv to start with %q, got %v", sbxBinary, commands.argv)
			}
			for _, want := range []string{sbxDetachedFlag, formatRunnerName(1, globalCfg.Runner.Harness)} {
				if !slices.Contains(commands.argv, want) {
					t.Errorf("expected the argv to contain %q, got %v", want, commands.argv)
				}
			}
			if !strings.Contains(commands.argv[len(commands.argv)-1], taskToRun.Title) {
				t.Errorf("expected the rendered prompt as the last argument, got %v", commands.argv)
			}

			if taskToRun.Status != task.StatusInProgress {
				t.Errorf("expected status %q, got %q", task.StatusInProgress, taskToRun.Status)
			}
			if taskToRun.RunnerID != 1 {
				t.Errorf("expected runner 1, got %d", taskToRun.RunnerID)
			}
			if taskToRun.RunnerSessionID != testCase.wantSessionID {
				t.Errorf("expected session %q, got %q", testCase.wantSessionID, taskToRun.RunnerSessionID)
			}
			if taskToRun.StartedAt.IsZero() {
				t.Error("expected started at to be stamped")
			}
		})
	}
}

func TestRunnerService_RunTask_SpawnFailureLeavesTheTaskAlone(t *testing.T) {
	taskToRun := &task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	}
	logger := common.NewLogger("")
	commands := &fakeCommandRunner{err: fmt.Errorf("sbx: no such binary")}
	service := New(
		logger,
		&config.LocalConfig{ProjectSlug: testProjectSlug},
		config.DefaultConfig(),
		task.NewTaskService(&fakeTaskRepo{tasks: []*task.Task{taskToRun}}, logger),
		commands,
	)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, false) })
	if err == nil {
		t.Fatal("expected the spawn failure to surface")
	}

	if taskToRun.Status != task.StatusTodo {
		t.Errorf("expected the task to stay %q, got %q", task.StatusTodo, taskToRun.Status)
	}
	if taskToRun.RunnerID != 0 {
		t.Errorf("expected no runner to be recorded, got %d", taskToRun.RunnerID)
	}
}

func TestRunnerService_RunTask_DryRunUsesConfiguredPromptFile(t *testing.T) {
	const promptFileName = "impl.md"
	setupPromptDirs(t)
	writePromptFile(t, common.LocalPromptsDir(), promptFileName, "custom prompt for {{taskTitle}}: {{taskDescription}}")

	taskToRun := &task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	}
	service := newTestServiceWithConfigs(
		&config.LocalConfig{ProjectSlug: testProjectSlug, PromptFile: promptFileName},
		config.DefaultConfig(),
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
	setupPromptDirs(t)
	writePromptFile(t, common.LocalPromptsDir(), promptFileName, "nothing to substitute here")

	service := newTestServiceWithConfigs(
		&config.LocalConfig{ProjectSlug: testProjectSlug, PromptFile: promptFileName},
		config.DefaultConfig(),
		&task.Task{
			ID:          "task-1",
			Title:       "Fix login",
			Description: "SSO is broken",
			Status:      task.StatusTodo,
			ProjectSlug: testProjectSlug,
		},
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

func TestRunnerService_AllocateRunnerID(t *testing.T) {
	inProgressOn := func(runnerID int) *task.Task {
		return &task.Task{
			ID:          task.TaskID(fmt.Sprintf("busy-%d", runnerID)),
			Status:      task.StatusInProgress,
			RunnerID:    runnerID,
			ProjectSlug: testProjectSlug,
		}
	}

	cases := []struct {
		name    string
		limit   int
		tasks   []*task.Task
		wantID  int
		wantErr bool
	}{
		{name: "empty pool takes the first slot", limit: 3, wantID: 1},
		{name: "takes the next free slot", limit: 3, tasks: []*task.Task{inProgressOn(1)}, wantID: 2},
		{name: "fills a gap left in the middle", limit: 3, tasks: []*task.Task{inProgressOn(1), inProgressOn(3)}, wantID: 2},
		{
			name:  "ignores tasks that are not in progress",
			limit: 3,
			tasks: []*task.Task{
				{ID: "done", Status: task.StatusDone, RunnerID: 1, ProjectSlug: testProjectSlug},
				{ID: "fucked-up", Status: task.StatusFuckedUp, RunnerID: 2, ProjectSlug: testProjectSlug},
			},
			wantID: 1,
		},
		{
			name:    "fails when every slot is taken",
			limit:   2,
			tasks:   []*task.Task{inProgressOn(1), inProgressOn(2)},
			wantErr: true,
		},
		{
			name:    "ignores slots above the limit but still fails when full",
			limit:   1,
			tasks:   []*task.Task{inProgressOn(1), inProgressOn(7)},
			wantErr: true,
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := newTestServiceWithConfigs(
				&config.LocalConfig{ProjectSlug: testProjectSlug, MaxConcurrentRunners: testCase.limit},
				config.DefaultConfig(),
				testCase.tasks...,
			)

			runnerID, err := service.allocateRunnerID(testProjectSlug)

			if testCase.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got runner %d", runnerID)
				}
				if !strings.Contains(err.Error(), config.MaxConcurrentRunnersKey) {
					t.Errorf("expected the error to name the config key, got %q", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if runnerID != testCase.wantID {
				t.Errorf("expected runner %d, got %d", testCase.wantID, runnerID)
			}
		})
	}
}

func TestRunnerService_RunTask_DryRunPrintsRunner(t *testing.T) {
	taskToRun := &task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	}
	busy := &task.Task{
		ID:          "task-0",
		Title:       "Already running",
		Status:      task.StatusInProgress,
		RunnerID:    1,
		ProjectSlug: testProjectSlug,
	}
	service := newTestService(taskToRun, busy)

	var err error
	out := captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, true) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(out, formatRunnerName(2, config.DefaultConfig().Runner.Harness)) {
		t.Errorf("expected dry run output to name the allocated runner, got %q", out)
	}
}

func TestRunnerService_RunTask_DryRunFailsWhenPoolIsFull(t *testing.T) {
	taskToRun := &task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	}
	busy := &task.Task{
		ID:          "task-0",
		Title:       "Already running",
		Status:      task.StatusInProgress,
		RunnerID:    1,
		ProjectSlug: testProjectSlug,
	}
	service := newTestServiceWithConfigs(
		&config.LocalConfig{ProjectSlug: testProjectSlug, MaxConcurrentRunners: 1},
		config.DefaultConfig(),
		taskToRun,
		busy,
	)

	var err error
	captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, true) })
	if err == nil {
		t.Fatal("expected an error when every runner of the project is busy")
	}
}

func TestRunnerService_RunTask_DryRunPrintsCommandWithoutRunningIt(t *testing.T) {
	taskToRun := &task.Task{
		ID:          "task-1",
		Title:       "Fix login",
		Description: "SSO is broken",
		Status:      task.StatusTodo,
		ProjectSlug: testProjectSlug,
	}
	logger := common.NewLogger("")
	globalCfg := config.DefaultConfig()
	commands := &fakeCommandRunner{}
	service := New(
		logger,
		&config.LocalConfig{ProjectSlug: testProjectSlug},
		globalCfg,
		task.NewTaskService(&fakeTaskRepo{tasks: []*task.Task{taskToRun}}, logger),
		commands,
	)

	var err error
	out := captureOutput(func() { err = service.RunTask(testProjectSlug, taskToRun.ID, true) })
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, want := range []string{sbxBinary, sbxDetachedFlag, formatRunnerName(1, globalCfg.Runner.Harness)} {
		if !strings.Contains(out, strconv.Quote(want)) {
			t.Errorf("expected dry run output to contain the argument %q, got %q", want, out)
		}
	}

	if commands.argv != nil {
		t.Errorf("expected a dry run not to run anything, got argv %v", commands.argv)
	}
}
