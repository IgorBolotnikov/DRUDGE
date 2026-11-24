package actions_test

import (
	"errors"
	"fmt"
	"slices"
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

func (fakeDrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return nil
}

func (fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, nil
}

func (fakeDrudgerService) EditTask(projectSlug string, id task.TaskID, changes task.EditTaskDto) (*task.Task, error) {
	return nil, errors.New("this test edits no task")
}

func (fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	return nil, nil, nil
}

func (fakeDrudgerService) ReclaimDrudgers(projectSlug string) ([]drudger.FreedSlot, error) {
	return nil, nil
}

func (fakeDrudgerService) NukeDrudger(projectSlug string, slot int, isForced bool) error {
	return nil
}

func (fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return nil, nil
}

// fakeAsker answers every question with isYes and records the questions.
type fakeAsker struct {
	isYes     bool
	questions []string
}

func (asker *fakeAsker) Confirm(question string) bool {
	asker.questions = append(asker.questions, question)
	return asker.isYes
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
	services := actions.Services{Drudgers: fakeDrudgerService{}, Asker: &fakeAsker{}}
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

// askingRun asks before it reruns and says what the answer was in its verdict.
func askingRun(services actions.Services, projectSlug string, command actions.Command, msg views.CommandMsg) tea.Msg {
	if !services.Asker.Confirm("Rerun " + command.Target + "?") {
		return actions.DoneMsg{Command: command, IsSkipped: true, Verdict: "kept"}
	}
	return actions.DoneMsg{Command: command, Verdict: "reran"}
}

func TestActions_RunAsksThroughTheAsker(t *testing.T) {
	cases := []struct {
		name        string
		isYes       bool
		wantVerdict string
	}{
		{name: "yes", isYes: true, wantVerdict: "reran"},
		{name: "no", isYes: false, wantVerdict: "kept"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			asker := &fakeAsker{isYes: testCase.isYes}
			kind := actions.Kind{Name: views.RerunTaskCommand, Run: askingRun}
			commands := actions.New(actions.Services{Drudgers: fakeDrudgerService{}, Asker: asker}, testProjectSlug, []actions.Kind{kind})

			_, cmd := commands.Start(views.RerunTaskMsg{TaskID: testTaskID})
			done := cmd().(actions.DoneMsg)

			if !slices.Equal(asker.questions, []string{"Rerun 3f9a1c2e?"}) {
				t.Errorf("expected one question, got %q", asker.questions)
			}
			if done.Verdict != testCase.wantVerdict {
				t.Errorf("expected verdict %q, got %q", testCase.wantVerdict, done.Verdict)
			}
		})
	}
}

func TestActions_Question(t *testing.T) {
	asking := actions.Kind{
		Name: views.RerunTaskCommand,
		Run:  askingRun,
		Question: func(command actions.Command, msg views.CommandMsg) (string, bool) {
			return "Start " + command.Target + " over?", true
		},
	}
	recorder := &recordingKind{}
	commands := actions.New(actions.Services{Drudgers: fakeDrudgerService{}}, testProjectSlug, []actions.Kind{asking, recorder.kind()})
	cases := []struct {
		name         string
		msg          views.CommandMsg
		wantQuestion string
		wantOK       bool
	}{
		{name: "a kind with a question asks it", msg: views.RerunTaskMsg{TaskID: testTaskID}, wantQuestion: "Start 3f9a1c2e over?", wantOK: true},
		{name: "a kind without a question starts right away", msg: views.RunTaskMsg{TaskID: testTaskID}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			question, ok := commands.Question(testCase.msg)
			if question != testCase.wantQuestion || ok != testCase.wantOK {
				t.Errorf("expected question %q (ok %v), got %q (ok %v)", testCase.wantQuestion, testCase.wantOK, question, ok)
			}
		})
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
