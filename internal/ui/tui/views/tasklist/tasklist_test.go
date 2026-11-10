package tasklist

import (
	"errors"
	"testing"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

func TestView(t *testing.T) {
	loaded := views.TasksLoadedMsg{Tasks: tuitest.ListedTasks()}
	cases := []struct {
		name     string
		width    int
		height   int
		messages []tea.Msg
	}{
		{name: "loading", width: 80, height: 4},
		{name: "empty", width: 80, height: 4, messages: []tea.Msg{views.TasksLoadedMsg{}}},
		{name: "error", width: 80, height: 4, messages: []tea.Msg{views.TasksLoadedMsg{Err: errors.New("permission denied")}}},
		{name: "loaded", width: 100, height: 8, messages: []tea.Msg{loaded}},
		{name: "moved_down", width: 100, height: 8, messages: []tea.Msg{loaded, tuitest.KeyDown, tuitest.KeyArrowDown, tuitest.KeyDown, tuitest.KeyUp}},
		{name: "scrolled_to_bottom", width: 100, height: 3, messages: []tea.Msg{loaded, tuitest.KeyBottom, tuitest.KeyDown}},
		{name: "back_to_top", width: 100, height: 3, messages: []tea.Msg{loaded, tuitest.KeyBottom, tuitest.KeyTop, tuitest.KeyUp}},
		{name: "narrow", width: 60, height: 8, messages: []tea.Msg{loaded}},
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
