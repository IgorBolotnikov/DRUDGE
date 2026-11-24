package tui

import (
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/spinner"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testQuestion = "Start 7c1e4f90 over? What its last run left behind is cleared."

var (
	keyYes       = tea.KeyPressMsg{Code: 'y', Text: "y"}
	keyNo        = tea.KeyPressMsg{Code: 'n', Text: "n"}
	keyStray     = tea.KeyPressMsg{Code: 'x', Text: "x"}
	keyInterrupt = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
)

// answerOf returns the answer waiting on a channel. ok is false while the
// channel holds none.
func answerOf(answer <-chan bool) (isYes bool, ok bool) {
	select {
	case isYes = <-answer:
		return isYes, true
	default:
		return false, false
	}
}

func TestConfirmOverlay_Answers(t *testing.T) {
	cases := []struct {
		name         string
		keys         []tea.Msg
		wantAnswered bool
		wantYes      bool
		wantOverlays []string
	}{
		{name: "y confirms", keys: []tea.Msg{keyYes}, wantAnswered: true, wantYes: true},
		{name: "n cancels", keys: []tea.Msg{keyNo}, wantAnswered: true},
		{name: "esc cancels", keys: []tea.Msg{tuitest.KeyEscape}, wantAnswered: true},
		{name: "a stray key does nothing", keys: []tea.Msg{keyStray}, wantOverlays: []string{confirmTitle}},
		{name: "q does nothing", keys: []tea.Msg{keyQuit}, wantOverlays: []string{confirmTitle}},
		{name: "? opens the help over it", keys: []tea.Msg{keyHelp}, wantOverlays: []string{confirmTitle, helpOverlayTitle}},
		{name: "the help closes back to it", keys: []tea.Msg{keyHelp, tuitest.KeyEscape}, wantOverlays: []string{confirmTitle}},
		{name: "quitting answers no", keys: []tea.Msg{keyInterrupt}, wantAnswered: true, wantOverlays: []string{confirmTitle}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			model := newCommandTestModel(t, &fakeCommandService{})
			answer := make(chan bool, 1)
			update(model, actions.ConfirmMsg{Question: testQuestion, Answer: answer})

			for _, keyPress := range testCase.keys {
				model.Update(keyPress)
			}

			isYes, isAnswered := answerOf(answer)
			if isAnswered != testCase.wantAnswered || isYes != testCase.wantYes {
				t.Errorf("expected answered %v and yes %v, got answered %v and yes %v", testCase.wantAnswered, testCase.wantYes, isAnswered, isYes)
			}
			if got := overlayTitles(model); !slices.Equal(got, testCase.wantOverlays) {
				t.Errorf("expected overlays %q, got %q", testCase.wantOverlays, got)
			}
		})
	}
}

func TestConfirmOverlay_ShowsQuestionAndKeys(t *testing.T) {
	model := newCommandTestModel(t, &fakeCommandService{})
	update(model, actions.ConfirmMsg{Question: testQuestion, Answer: make(chan bool, 1)})

	screen := ansi.Strip(model.View().Content)

	for _, want := range []string{testQuestion, "y yes", "n/esc no"} {
		if !strings.Contains(screen, want) {
			t.Errorf("expected the screen to show %q, got\n%s", want, screen)
		}
	}
}

// pressCommandKey presses a key of a View that asks for a command and
// returns the command the root returns for the command message, without
// running it.
func pressCommandKey(t *testing.T, model *Model, keyPress tea.Msg) tea.Cmd {
	t.Helper()
	_, cmd := model.Update(keyPress)
	commandMsg, ok := findCommandMsg(model, cmd)
	if !ok {
		t.Fatalf("expected key %v to ask for a command", keyPress)
	}
	return startHeld(model, commandMsg)
}

// findCommandMsg runs a command and the commands its messages lead to until
// one of them sends a command message.
func findCommandMsg(model *Model, cmd tea.Cmd) (views.CommandMsg, bool) {
	if cmd == nil {
		return nil, false
	}
	switch msg := cmd().(type) {
	case views.CommandMsg:
		return msg, true
	case tea.BatchMsg:
		for _, batched := range msg {
			if commandMsg, ok := findCommandMsg(model, batched); ok {
				return commandMsg, true
			}
		}
	case feeds.TickMsg, spinner.TickMsg:
	default:
		_, next := model.Update(msg)
		return findCommandMsg(model, next)
	}
	return nil, false
}

func TestModel_RerunsTaskOnConfirm(t *testing.T) {
	errorLine := "✗ " + errNotTodo.Error()
	cases := []struct {
		name         string
		answerKey    tea.Msg
		wantRerunIDs []task.TaskID
		wantLine     string
	}{
		{name: "a yes reruns the task", answerKey: keyYes, wantRerunIDs: []task.TaskID{runTaskID}, wantLine: "✓ 7c1e4f90 handed to Drudger 3"},
		{name: "a no runs nothing and leaves the bar", answerKey: keyNo, wantLine: errorLine},
		{name: "esc runs nothing and leaves the bar", answerKey: tuitest.KeyEscape, wantLine: errorLine},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			service := &fakeCommandService{runErr: errNotTodo}
			model := newCommandTestModel(t, service)
			recordTimers(model)
			update(model, views.RunTaskMsg{TaskID: otherRunTaskID})

			pressKeys(model, []tea.Msg{tuitest.KeyDown, tuitest.KeyDown})
			waitAnswer := pressCommandKey(t, model, tuitest.KeyRerun)
			if got := overlayTitles(model); !slices.Equal(got, []string{confirmTitle}) {
				t.Fatalf("expected R to open the confirm overlay, got overlays %q", got)
			}
			if screen := ansi.Strip(model.View().Content); !strings.Contains(screen, testQuestion) {
				t.Errorf("expected the overlay to ask %q, got\n%s", testQuestion, screen)
			}
			if commandLine(model) != errorLine {
				t.Errorf("expected the question to leave the bar, got %q", commandLine(model))
			}

			pressKeys(model, []tea.Msg{testCase.answerKey})
			runCommand(model, waitAnswer)

			if !slices.Equal(service.rerunIDs, testCase.wantRerunIDs) {
				t.Errorf("expected reruns %v, got %v", testCase.wantRerunIDs, service.rerunIDs)
			}
			if got := commandLine(model); got != testCase.wantLine {
				t.Errorf("expected line %q, got %q", testCase.wantLine, got)
			}
			if got := overlayTitles(model); len(got) != 0 {
				t.Errorf("expected the answer to close the overlay, got overlays %q", got)
			}
		})
	}
}

func TestModel_RefusesCommandWithQuestionWhileOneRuns(t *testing.T) {
	model := newCommandTestModel(t, &fakeCommandService{})
	recordTimers(model)
	startHeld(model, views.RunTaskMsg{TaskID: otherRunTaskID})

	update(model, views.RerunTaskMsg{TaskID: runTaskID})

	if got := overlayTitles(model); len(got) != 0 {
		t.Errorf("expected no question while a command runs, got overlays %q", got)
	}
	if got, want := commandLine(model), "! wait, still running run a01f3b55"; got != want {
		t.Errorf("expected line %q, got %q", want, got)
	}
}
