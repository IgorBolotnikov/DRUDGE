package drudgerlist

import (
	"errors"
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

var errPermissionDenied = errors.New("permission denied")

var readAt = time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)

// testDrudgers returns a working Drudger, an idle one, one whose agent was
// refused, one whose sandbox is gone and one DRUDGE has not looked at.
func testDrudgers() []*drudger.Drudger {
	return []*drudger.Drudger{
		{Slot: 1, TaskID: "3f9a1c2e-0000-0000-0000-000000000001", SandboxHealth: drudger.SandboxUsable, WorkspaceHealth: drudger.WorkspaceUsable, AgentHealth: drudger.AgentReady, LastChecked: readAt.Add(-12 * time.Second)},
		{Slot: 2, SandboxHealth: drudger.SandboxUsable, WorkspaceHealth: drudger.WorkspaceUsable, AgentHealth: drudger.AgentReady, LastChecked: readAt.Add(-5 * time.Minute)},
		{Slot: 3, SandboxHealth: drudger.SandboxUsable, WorkspaceHealth: drudger.WorkspaceUsable, AgentHealth: drudger.AgentRefused, LastChecked: readAt.Add(-3 * time.Hour)},
		{Slot: 4, TaskID: "9d8b2a11-0000-0000-0000-000000000004", SandboxHealth: drudger.SandboxGone, AgentHealth: drudger.AgentReady, LastChecked: readAt.Add(-50 * time.Hour)},
		{Slot: 5},
	}
}

// reversedDrudgers returns the test Drudgers in reverse order.
func reversedDrudgers() []*drudger.Drudger {
	drudgers := testDrudgers()
	slices.Reverse(drudgers)
	return drudgers
}

// drudgersWithoutSlot returns the test Drudgers without the Drudger of slot.
func drudgersWithoutSlot(slot int) []*drudger.Drudger {
	return slices.DeleteFunc(testDrudgers(), func(entry *drudger.Drudger) bool { return entry.Slot == slot })
}

func TestView(t *testing.T) {
	loaded := views.DrudgersLoadedMsg{Drudgers: testDrudgers(), ReadAt: readAt}
	cases := []struct {
		name     string
		width    int
		height   int
		messages []tea.Msg
	}{
		{name: "loading", width: 80, height: 4},
		{name: "empty", width: 80, height: 4, messages: []tea.Msg{views.DrudgersLoadedMsg{ReadAt: readAt}}},
		{name: "error", width: 80, height: 4, messages: []tea.Msg{views.DrudgersLoadedMsg{Err: errPermissionDenied}}},
		{name: "error_keeps_drudgers", width: 80, height: 8, messages: []tea.Msg{loaded, views.DrudgersLoadedMsg{Err: errPermissionDenied}}},
		{name: "wide", width: 80, height: 8, messages: []tea.Msg{loaded}},
		{name: "narrow", width: 30, height: 8, messages: []tea.Msg{loaded}},
		{name: "narrowest", width: 12, height: 8, messages: []tea.Msg{loaded}},
		{name: "moved_down", width: 80, height: 8, messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyArrowDown, tuitest.KeyDown, tuitest.KeyUp}},
		{name: "scrolled_down", width: 80, height: 3, messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyArrowDown}},
		{name: "scrolled_to_bottom", width: 80, height: 3, messages: []tea.Msg{loaded, tuitest.KeyBottom, tuitest.KeyDown}},
		{name: "back_to_top", width: 80, height: 3, messages: []tea.Msg{loaded, tuitest.KeyBottom, tuitest.KeyTop, tuitest.KeyUp}},
		{name: "clicked", width: 80, height: 8, messages: []tea.Msg{loaded, tea.MouseClickMsg{Y: 4, Button: tea.MouseLeft}}},
		{name: "clicked_on_header", width: 80, height: 8, messages: []tea.Msg{loaded, tuitest.KeyDown, tea.MouseClickMsg{Y: 0, Button: tea.MouseLeft}}},
		{name: "wheel_moves_cursor", width: 80, height: 3, messages: []tea.Msg{loaded, tea.MouseWheelMsg{Button: tea.MouseWheelDown}, tea.MouseWheelMsg{Button: tea.MouseWheelDown}}},
		{name: "wheel_back_up", width: 80, height: 3, messages: []tea.Msg{loaded, tuitest.KeyBottom, tea.MouseWheelMsg{Button: tea.MouseWheelUp}, tea.MouseWheelMsg{Button: tea.MouseWheelUp}}},
		{name: "reload_keeps_cursor_on_slot", width: 80, height: 8, messages: []tea.Msg{loaded, tuitest.KeyDown, views.DrudgersLoadedMsg{Drudgers: reversedDrudgers(), ReadAt: readAt}}},
		{name: "reload_without_cursor_slot", width: 80, height: 8, messages: []tea.Msg{loaded, tuitest.KeyBottom, views.DrudgersLoadedMsg{Drudgers: drudgersWithoutSlot(5), ReadAt: readAt}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(testCase.width, testCase.height)
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			tuitest.AssertGolden(t, testCase.name, view.Render())
		})
	}
}

func TestView_Status(t *testing.T) {
	loaded := views.DrudgersLoadedMsg{Drudgers: testDrudgers(), ReadAt: readAt}
	cases := []struct {
		name     string
		messages []tea.Msg
		want     string
	}{
		{name: "loading", want: "not checked yet"},
		{name: "never checked", messages: []tea.Msg{loaded}, want: "slot 1/5 · not checked yet"},
		{name: "no Drudgers", messages: []tea.Msg{views.DrudgersLoadedMsg{ReadAt: readAt}, views.DrudgersCheckedMsg{CheckedAt: readAt}}, want: "checked 0s ago"},
		{name: "checked", messages: []tea.Msg{views.DrudgersCheckedMsg{CheckedAt: readAt}}, want: "checked 0s ago"},
		{name: "a read counts from the check", messages: []tea.Msg{views.DrudgersCheckedMsg{CheckedAt: readAt.Add(-12 * time.Second)}, loaded}, want: "slot 1/5 · checked 12s ago"},
		{name: "minutes", messages: []tea.Msg{loaded, views.DrudgersCheckedMsg{CheckedAt: readAt.Add(-2 * time.Minute)}}, want: "slot 1/5 · checked 2m ago"},
		{name: "a failed check keeps the last check", messages: []tea.Msg{views.DrudgersCheckedMsg{CheckedAt: readAt.Add(-30 * time.Second)}, views.DrudgersCheckedMsg{Err: errPermissionDenied}, loaded}, want: "slot 1/5 · checked 30s ago"},
		{name: "cursor moved", messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyDown}, want: "slot 3/5 · not checked yet"},
		{name: "cursor at the bottom", messages: []tea.Msg{loaded, tuitest.KeyBottom}, want: "slot 5/5 · not checked yet"},
		{name: "reload keeps the cursor on its slot", messages: []tea.Msg{loaded, tuitest.KeyDown, views.DrudgersLoadedMsg{Drudgers: reversedDrudgers(), ReadAt: readAt}}, want: "slot 4/5 · not checked yet"},
		{name: "reload without the cursor slot keeps the row", messages: []tea.Msg{loaded, tuitest.KeyDown, views.DrudgersLoadedMsg{Drudgers: drudgersWithoutSlot(2), ReadAt: readAt}}, want: "slot 2/4 · not checked yet"},
		{name: "reload without the last slot moves up", messages: []tea.Msg{loaded, tuitest.KeyBottom, views.DrudgersLoadedMsg{Drudgers: drudgersWithoutSlot(5), ReadAt: readAt}}, want: "slot 4/4 · not checked yet"},
		{name: "error keeps the cursor", messages: []tea.Msg{loaded, tuitest.KeyBottom, views.DrudgersLoadedMsg{Err: errPermissionDenied}}, want: "slot 5/5 · not checked yet"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(80, 8)
			for _, msg := range testCase.messages {
				view.Update(msg)
			}
			if got := view.Status(); got != testCase.want {
				t.Errorf("expected status %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestView_SelectsAndOpensTasks(t *testing.T) {
	loaded := views.DrudgersLoadedMsg{Drudgers: testDrudgers(), ReadAt: readAt}
	firstTaskID := testDrudgers()[0].TaskID
	fourthTaskID := testDrudgers()[3].TaskID
	cases := []struct {
		name     string
		messages []tea.Msg
		input    tea.Msg
		want     []tea.Msg
	}{
		{name: "cursor moves onto an idle Drudger", messages: []tea.Msg{loaded}, input: tuitest.KeyDown},
		{name: "cursor moves onto a busy Drudger", messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyDown}, input: tuitest.KeyDown, want: []tea.Msg{views.TaskSelectedMsg{TaskID: fourthTaskID}}},
		{name: "cursor jumps onto a busy Drudger", messages: []tea.Msg{loaded, tuitest.KeyBottom}, input: tuitest.KeyTop, want: []tea.Msg{views.TaskSelectedMsg{TaskID: firstTaskID}}},
		{name: "cursor stays at the top", messages: []tea.Msg{loaded}, input: tuitest.KeyUp},
		{name: "no Drudgers to move over", messages: []tea.Msg{views.DrudgersLoadedMsg{ReadAt: readAt}}, input: tuitest.KeyDown},
		{name: "click moves the cursor onto a busy Drudger", messages: []tea.Msg{loaded}, input: tea.MouseClickMsg{Y: 4, Button: tea.MouseLeft}, want: []tea.Msg{views.TaskSelectedMsg{TaskID: fourthTaskID}}},
		{name: "click on the cursor row", messages: []tea.Msg{loaded}, input: tea.MouseClickMsg{Y: 1, Button: tea.MouseLeft}},
		{name: "click on an idle Drudger", messages: []tea.Msg{loaded}, input: tea.MouseClickMsg{Y: 2, Button: tea.MouseLeft}},
		{name: "wheel moves the cursor onto a busy Drudger", messages: []tea.Msg{loaded, tea.WindowSizeMsg{Width: 80, Height: 2}, tea.MouseWheelMsg{Button: tea.MouseWheelDown}, tea.MouseWheelMsg{Button: tea.MouseWheelDown}}, input: tea.MouseWheelMsg{Button: tea.MouseWheelDown}, want: []tea.Msg{views.TaskSelectedMsg{TaskID: fourthTaskID}}},
		{name: "enter opens the task of the Drudger under the cursor", messages: []tea.Msg{loaded}, input: tuitest.KeyEnter, want: []tea.Msg{views.OpenTaskMsg{TaskID: firstTaskID}}},
		{name: "enter on an idle Drudger does nothing", messages: []tea.Msg{loaded, tuitest.KeyDown}, input: tuitest.KeyEnter},
		{name: "enter with no Drudgers does nothing", messages: []tea.Msg{views.DrudgersLoadedMsg{ReadAt: readAt}}, input: tuitest.KeyEnter},
		{name: "L opens the runlog of the task of the Drudger under the cursor", messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyDown, tuitest.KeyDown}, input: tuitest.KeyRunLog, want: []tea.Msg{views.OpenRunLogMsg{TaskID: fourthTaskID}}},
		{name: "L on an idle Drudger does nothing", messages: []tea.Msg{loaded, tuitest.KeyBottom}, input: tuitest.KeyRunLog},
		{name: "L with no Drudgers does nothing", messages: []tea.Msg{views.DrudgersLoadedMsg{ReadAt: readAt}}, input: tuitest.KeyRunLog},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			view := New(tuitest.Styles())
			view.SetSize(80, 8)
			for _, msg := range testCase.messages {
				if size, ok := msg.(tea.WindowSizeMsg); ok {
					view.SetSize(size.Width, size.Height)
					continue
				}
				view.Update(msg)
			}
			got := tuitest.Messages(view.Update(testCase.input))
			if !slices.Equal(got, testCase.want) {
				t.Errorf("expected messages %v, got %v", testCase.want, got)
			}
		})
	}
}
