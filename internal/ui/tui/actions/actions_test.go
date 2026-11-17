package actions_test

import (
	"fmt"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "3f9a1c2e-0000-4000-8000-000000000001"

type fakeDrudgerService struct{}

func (fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return nil
}

func (fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, nil
}

func (fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	return nil, nil, nil
}

func (fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return nil, nil
}

// recordingKind runs the run command and records what each run got.
type recordingKind struct {
	runs []recordedRun
}

type recordedRun struct {
	services    actions.Services
	projectSlug string
	command     actions.Command
	msg         views.CommandMsg
}

func (recorder *recordingKind) kind() actions.Kind {
	return actions.Kind{Name: views.RunTaskCommand, Run: recorder.run}
}

func (recorder *recordingKind) run(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	recorder.runs = append(recorder.runs, recordedRun{services: services, projectSlug: projectSlug, command: command, msg: msg})
	return actions.DoneMsg{Command: command, Verdict: "ran"}
}

func TestActions_Start(t *testing.T) {
	recorder := &recordingKind{}
	services := actions.Services{Drudgers: fakeDrudgerService{}}
	commands := actions.New(services, testProjectSlug, []actions.Kind{recorder.kind()})
	msg := views.RunTaskMsg{TaskID: testTaskID}

	command, cmd := commands.Start(msg)

	wantCommand := actions.Command{Name: views.RunTaskCommand, Target: "3f9a1c2e"}
	if command != wantCommand {
		t.Errorf("expected command %v, got %v", wantCommand, command)
	}
	if len(recorder.runs) != 0 {
		t.Fatal("expected the run to wait for its command")
	}
	done, ok := cmd().(actions.DoneMsg)
	if !ok {
		t.Fatal("expected a done message")
	}
	if done.Verdict != "ran" {
		t.Errorf("expected the done message of the kind, got %+v", done)
	}
	wantRun := recordedRun{services: services, projectSlug: testProjectSlug, command: wantCommand, msg: msg}
	if len(recorder.runs) != 1 || recorder.runs[0] != wantRun {
		t.Errorf("expected one run %+v, got %+v", wantRun, recorder.runs)
	}
}

func TestActions_StartPanicsOnUnknownCommand(t *testing.T) {
	commands := actions.New(actions.Services{Drudgers: fakeDrudgerService{}}, testProjectSlug, nil)
	defer func() {
		recovered := recover()
		want := `a view asked for command "run", which no command kind runs`
		if fmt.Sprint(recovered) != want {
			t.Errorf("expected panic %q, got %v", want, recovered)
		}
	}()
	commands.Start(views.RunTaskMsg{TaskID: testTaskID})
}

func TestCommand_String(t *testing.T) {
	command := actions.Command{Name: views.RunTaskCommand, Target: "3f9a1c2e"}
	if got := command.String(); got != "run 3f9a1c2e" {
		t.Errorf("expected run 3f9a1c2e, got %q", got)
	}
}
