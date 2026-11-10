package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/tuitest"
)

// sizeView is a View that shows its name and the size it got, so a golden
// shows how a layout divides the space.
type sizeView struct {
	name          string
	width         int
	height        int
	receivedCount int
}

func (view *sizeView) Name() string { return view.name }

func (view *sizeView) SetSize(width, height int) {
	view.width = width
	view.height = height
}

func (view *sizeView) Update(msg tea.Msg) tea.Cmd {
	view.receivedCount++
	return nil
}

func (view *sizeView) Render() string {
	return fmt.Sprintf("%dx%d", view.width, view.height)
}

func (view *sizeView) KeyHints() []key.Binding { return nil }

// sizeTile returns a Tile with a sizeView of the name.
func sizeTile(name string, weight float64) *Tile {
	return NewTile(&sizeView{name: name}, tuitest.Styles(), weight)
}

func TestTab_Render(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
		layout func() Node
	}{
		{name: "layout_single_tile", width: 40, height: 6, layout: func() Node {
			return sizeTile("a", 1)
		}},
		{name: "layout_side_by_side", width: 40, height: 6, layout: func() Node {
			return NewSplit(SideBySide, 1, sizeTile("a", 1), sizeTile("b", 1))
		}},
		{name: "layout_side_by_side_odd_width", width: 41, height: 6, layout: func() Node {
			return NewSplit(SideBySide, 1, sizeTile("a", 1), sizeTile("b", 1), sizeTile("c", 1))
		}},
		{name: "layout_stacked_weights", width: 40, height: 12, layout: func() Node {
			return NewSplit(Stacked, 1, sizeTile("a", 2), sizeTile("b", 1))
		}},
		{name: "layout_fractional_weights", width: 40, height: 6, layout: func() Node {
			return NewSplit(SideBySide, 1, sizeTile("a", 0.5), sizeTile("b", 1.5))
		}},
		{name: "layout_nested", width: 60, height: 12, layout: func() Node {
			return NewSplit(Stacked, 1,
				NewSplit(SideBySide, 1, sizeTile("a", 2), sizeTile("b", 1)),
				NewSplit(SideBySide, 1, sizeTile("c", 1), sizeTile("d", 1)),
			)
		}},
		{name: "layout_too_short_for_every_tile", width: 40, height: 5, layout: func() Node {
			return NewSplit(Stacked, 1, sizeTile("a", 1), sizeTile("b", 1), sizeTile("c", 1), sizeTile("d", 1), sizeTile("e", 1), sizeTile("f", 1))
		}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tab, err := NewTab(testCase.name, testCase.layout())
			if err != nil {
				t.Fatal(err)
			}
			tab.SetSize(testCase.width, testCase.height)

			screen := tab.Render()
			lines := strings.Split(screen, "\n")
			if len(lines) != testCase.height {
				t.Errorf("expected %d lines, got %d", testCase.height, len(lines))
			}
			tuitest.AssertGolden(t, testCase.name, screen)
		})
	}
}

func TestNewTab_RefusesLayout(t *testing.T) {
	cases := []struct {
		name      string
		layout    Node
		wantError string
	}{
		{name: "no layout", layout: nil, wantError: "empty node"},
		{name: "split with one tile", layout: NewSplit(SideBySide, 1, sizeTile("a", 1)), wantError: "needs at least 2"},
		{name: "split with no tiles", layout: NewSplit(Stacked, 1), wantError: "needs at least 2"},
		{name: "unknown arrangement", layout: NewSplit("diagonal", 1, sizeTile("a", 1), sizeTile("b", 1)), wantError: `unknown arrangement "diagonal"`},
		{name: "zero tile weight", layout: NewSplit(Stacked, 1, sizeTile("a", 0), sizeTile("b", 1)), wantError: "weight 0 is not above 0"},
		{name: "negative split weight", layout: NewSplit(Stacked, 1, NewSplit(SideBySide, -1, sizeTile("a", 1), sizeTile("b", 1)), sizeTile("c", 1)), wantError: "weight -1 is not above 0"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewTab("Broken", testCase.layout)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), `tab "Broken"`) {
				t.Errorf("expected the error to name the tab, got %q", err)
			}
			if !strings.Contains(err.Error(), testCase.wantError) {
				t.Errorf("expected the error to contain %q, got %q", testCase.wantError, err)
			}
		})
	}
}

func TestNewTab_FocusesFirstTile(t *testing.T) {
	first := sizeTile("a", 1)
	tab, err := NewTab("Tab", NewSplit(Stacked, 1, NewSplit(SideBySide, 1, first, sizeTile("b", 1)), sizeTile("c", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if tab.FocusedTile() != first {
		t.Errorf("expected focus on the first tile, got the tile of %q", tab.FocusedTile().view.Name())
	}
}

func TestTab_BroadcastReachesEveryTile(t *testing.T) {
	views := []*sizeView{{name: "a"}, {name: "b"}, {name: "c"}}
	screenStyles := tuitest.Styles()
	tab, err := NewTab("Tab", NewSplit(Stacked, 1,
		NewSplit(SideBySide, 1, NewTile(views[0], screenStyles, 1), NewTile(views[1], screenStyles, 1)),
		NewTile(views[2], screenStyles, 1),
	))
	if err != nil {
		t.Fatal(err)
	}
	tab.Broadcast("message")
	for _, view := range views {
		if view.receivedCount != 1 {
			t.Errorf("expected view %q to get the message once, got it %d times", view.name, view.receivedCount)
		}
	}
}
