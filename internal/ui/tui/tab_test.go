package tui

import (
	"fmt"
	"slices"
	"strings"
	"testing"

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

func (view *sizeView) TakesAllKeys() bool { return false }

func (view *sizeView) Status() string { return view.name }

// sizeTile returns a Tile with a sizeView of the name.
func sizeTile(name string, weight float64) *Tile {
	return NewTile(&sizeView{name: name}, "", tuitest.Styles(), weight, true)
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

func TestTab_HasRoomForTiles(t *testing.T) {
	cases := []struct {
		name        string
		width       int
		height      int
		wantHasRoom bool
	}{
		{name: "every tile at the minimum", width: 24, height: 10, wantHasRoom: true},
		{name: "one column short", width: 23, height: 10, wantHasRoom: false},
		{name: "one row short", width: 24, height: 9, wantHasRoom: false},
		{name: "no size", width: 0, height: 0, wantHasRoom: false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tab, err := NewTab("Tab", NewSplit(Stacked, 1,
				NewSplit(SideBySide, 1, sizeTile("a", 1), sizeTile("b", 1)),
				sizeTile("c", 1),
			))
			if err != nil {
				t.Fatal(err)
			}
			tab.SetSize(testCase.width, testCase.height)
			if got := tab.HasRoomForTiles(); got != testCase.wantHasRoom {
				t.Errorf("expected room for tiles %v, got %v", testCase.wantHasRoom, got)
			}
		})
	}
}

func TestTab_RenderBeforeSetSize(t *testing.T) {
	tab, err := NewTab("Tab", NewSplit(Stacked, 1, sizeTile("a", 1), sizeTile("b", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if got := tab.Render(); got != "" {
		t.Errorf("expected nothing before the tab gets a size, got %q", got)
	}
}

func TestNewTab_RefusesLayout(t *testing.T) {
	cases := []struct {
		name       string
		layout     Node
		wantPath   string
		wantReason string
	}{
		{name: "no layout", layout: nil, wantPath: "layout: ", wantReason: "the node is empty"},
		{name: "split with one tile", layout: NewSplit(SideBySide, 1, sizeTile("a", 1)), wantPath: "layout.children: ", wantReason: "needs at least 2"},
		{name: "split with no tiles", layout: NewSplit(Stacked, 1), wantPath: "layout.children: ", wantReason: "needs at least 2"},
		{name: "unknown arrangement", layout: NewSplit("diagonal", 1, sizeTile("a", 1), sizeTile("b", 1)), wantPath: "layout.arrangement: ", wantReason: `unknown arrangement "diagonal"`},
		{name: "zero tile weight", layout: NewSplit(Stacked, 1, sizeTile("a", 0), sizeTile("b", 1)), wantPath: "layout.children[0].weight: ", wantReason: "0 is not above 0"},
		{name: "negative nested split weight", layout: NewSplit(Stacked, 1, sizeTile("a", 1), NewSplit(SideBySide, -1, sizeTile("b", 1), sizeTile("c", 1))), wantPath: "layout.children[1].weight: ", wantReason: "-1 is not above 0"},
		{name: "tile deep in the tree", layout: NewSplit(Stacked, 1, sizeTile("a", 1), NewSplit(SideBySide, 1, sizeTile("b", 1), sizeTile("c", -2))), wantPath: "layout.children[1].children[1].weight: ", wantReason: "-2 is not above 0"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := NewTab("Broken", testCase.layout)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.HasPrefix(err.Error(), testCase.wantPath) {
				t.Errorf("expected the error to start with %q, got %q", testCase.wantPath, err)
			}
			if !strings.Contains(err.Error(), testCase.wantReason) {
				t.Errorf("expected the error to contain %q, got %q", testCase.wantReason, err)
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
		NewSplit(SideBySide, 1, NewTile(views[0], "", screenStyles, 1, true), NewTile(views[1], "", screenStyles, 1, true)),
		NewTile(views[2], "", screenStyles, 1, true),
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

func TestTab_Statuses(t *testing.T) {
	cases := []struct {
		name          string
		names         []string
		hiddenIndexes []int
		want          []string
	}{
		{name: "every tile in tree order", names: []string{"a", "b", "c"}, want: []string{"a", "b", "c"}},
		{name: "a tile that hides its status", names: []string{"a", "b", "c"}, hiddenIndexes: []int{1}, want: []string{"a", "c"}},
		{name: "an empty status", names: []string{"a", "", "c"}, want: []string{"a", "c"}},
		{name: "every tile hides its status", names: []string{"a", "b", "c"}, hiddenIndexes: []int{0, 1, 2}, want: nil},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			tiles := make([]*Tile, 0, len(testCase.names))
			for index, name := range testCase.names {
				isStatusShown := !slices.Contains(testCase.hiddenIndexes, index)
				tiles = append(tiles, NewTile(&sizeView{name: name}, "", tuitest.Styles(), 1, isStatusShown))
			}
			tab, err := NewTab("Tab", NewSplit(Stacked, 1, NewSplit(SideBySide, 1, tiles[0], tiles[1]), tiles[2]))
			if err != nil {
				t.Fatal(err)
			}
			if got := tab.Statuses(); !slices.Equal(got, testCase.want) {
				t.Errorf("expected statuses %q, got %q", testCase.want, got)
			}
		})
	}
}
