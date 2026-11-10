package drudgerlist

import (
	"errors"
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
		{name: "scrolled_down", width: 80, height: 3, messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyArrowDown}},
		{name: "scrolled_to_bottom", width: 80, height: 3, messages: []tea.Msg{loaded, tuitest.KeyBottom, tuitest.KeyDown}},
		{name: "back_to_top", width: 80, height: 3, messages: []tea.Msg{loaded, tuitest.KeyBottom, tuitest.KeyTop, tuitest.KeyUp}},
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
		{name: "never checked", messages: []tea.Msg{loaded}, want: "not checked yet"},
		{name: "checked", messages: []tea.Msg{views.DrudgersCheckedMsg{CheckedAt: readAt}}, want: "checked 0s ago"},
		{name: "a read counts from the check", messages: []tea.Msg{views.DrudgersCheckedMsg{CheckedAt: readAt.Add(-12 * time.Second)}, loaded}, want: "checked 12s ago"},
		{name: "minutes", messages: []tea.Msg{loaded, views.DrudgersCheckedMsg{CheckedAt: readAt.Add(-2 * time.Minute)}}, want: "checked 2m ago"},
		{name: "a failed check keeps the last check", messages: []tea.Msg{views.DrudgersCheckedMsg{CheckedAt: readAt.Add(-30 * time.Second)}, views.DrudgersCheckedMsg{Err: errPermissionDenied}, loaded}, want: "checked 30s ago"},
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
