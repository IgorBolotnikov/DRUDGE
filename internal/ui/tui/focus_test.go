package tui

import (
	"slices"
	"testing"
)

// tallLeftLayout is one tall Tile on the left and three stacked on the right.
func tallLeftLayout() Node {
	return NewSplit(SideBySide, 1,
		sizeTile("left", 1),
		NewSplit(Stacked, 1, sizeTile("top", 1), sizeTile("middle", 1), sizeTile("bottom", 1)),
	)
}

func TestTileRects(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
		layout func() Node
		want   []Rect
	}{
		{name: "single tile", width: 40, height: 6, layout: func() Node {
			return sizeTile("a", 1)
		}, want: []Rect{{X: 0, Y: 0, Width: 40, Height: 6}}},
		{name: "side by side odd width", width: 41, height: 6, layout: func() Node {
			return NewSplit(SideBySide, 1, sizeTile("a", 1), sizeTile("b", 1), sizeTile("c", 1))
		}, want: []Rect{{X: 0, Y: 0, Width: 14, Height: 6}, {X: 14, Y: 0, Width: 13, Height: 6}, {X: 27, Y: 0, Width: 14, Height: 6}}},
		{name: "stacked weights", width: 40, height: 12, layout: func() Node {
			return NewSplit(Stacked, 1, sizeTile("a", 2), sizeTile("b", 1))
		}, want: []Rect{{X: 0, Y: 0, Width: 40, Height: 8}, {X: 0, Y: 8, Width: 40, Height: 4}}},
		{name: "nested", width: 60, height: 12, layout: func() Node {
			return NewSplit(Stacked, 1,
				NewSplit(SideBySide, 1, sizeTile("a", 2), sizeTile("b", 1)),
				NewSplit(SideBySide, 1, sizeTile("c", 1), sizeTile("d", 1)),
			)
		}, want: []Rect{{X: 0, Y: 0, Width: 40, Height: 6}, {X: 40, Y: 0, Width: 20, Height: 6}, {X: 0, Y: 6, Width: 30, Height: 6}, {X: 30, Y: 6, Width: 30, Height: 6}}},
		{name: "tall tile on the left", width: 60, height: 22, layout: tallLeftLayout, want: []Rect{
			{X: 0, Y: 0, Width: 30, Height: 22},
			{X: 30, Y: 0, Width: 30, Height: 7},
			{X: 30, Y: 7, Width: 30, Height: 8},
			{X: 30, Y: 15, Width: 30, Height: 7},
		}},
		{name: "too short for every tile", width: 40, height: 2, layout: func() Node {
			return NewSplit(Stacked, 1, sizeTile("a", 1), sizeTile("b", 1), sizeTile("c", 1), sizeTile("d", 1))
		}, want: []Rect{{X: 0, Y: 0, Width: 40, Height: 1}, {X: 0, Y: 1, Width: 40, Height: 0}, {X: 0, Y: 1, Width: 40, Height: 1}, {X: 0, Y: 2, Width: 40, Height: 0}}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			layout := testCase.layout()
			got := TileRects(layout, testCase.width, testCase.height)
			if !slices.Equal(got, testCase.want) {
				t.Errorf("expected rects %v, got %v", testCase.want, got)
			}

			layout.SetSize(testCase.width, testCase.height)
			for index, tile := range layout.Tiles() {
				rect := got[index]
				if tile.width != rect.Width || tile.height != rect.Height {
					t.Errorf("tile %d got %dx%d from the layout and %dx%d from its rect", index, tile.width, tile.height, rect.Width, rect.Height)
				}
			}
		})
	}
}

func TestNearestTile(t *testing.T) {
	tallLeft := TileRects(tallLeftLayout(), 60, 22)
	grid := []Rect{
		{X: 0, Y: 0, Width: 40, Height: 6}, {X: 40, Y: 0, Width: 20, Height: 6},
		{X: 0, Y: 6, Width: 30, Height: 6}, {X: 30, Y: 6, Width: 30, Height: 6},
	}
	withEmptyTile := []Rect{
		{X: 0, Y: 0, Width: 40, Height: 1}, {X: 0, Y: 1, Width: 40, Height: 0}, {X: 0, Y: 1, Width: 40, Height: 1},
	}
	cases := []struct {
		name      string
		rects     []Rect
		fromIndex int
		direction Direction
		want      int
	}{
		{name: "tall left tile right to the middle of three", rects: tallLeft, fromIndex: 0, direction: Right, want: 2},
		{name: "top right left to the tall tile", rects: tallLeft, fromIndex: 1, direction: Left, want: 0},
		{name: "bottom right left to the tall tile", rects: tallLeft, fromIndex: 3, direction: Left, want: 0},
		{name: "top right down to the middle", rects: tallLeft, fromIndex: 1, direction: Down, want: 2},
		{name: "middle right down to the bottom", rects: tallLeft, fromIndex: 2, direction: Down, want: 3},
		{name: "bottom right up to the middle", rects: tallLeft, fromIndex: 3, direction: Up, want: 2},
		{name: "tall left tile has nothing above", rects: tallLeft, fromIndex: 0, direction: Up, want: 0},
		{name: "tall left tile has nothing to the left", rects: tallLeft, fromIndex: 0, direction: Left, want: 0},
		{name: "top right has nothing to the right", rects: tallLeft, fromIndex: 1, direction: Right, want: 1},
		{name: "wide top left down to the tile below its center", rects: grid, fromIndex: 0, direction: Down, want: 2},
		{name: "narrow top right down to the tile below it", rects: grid, fromIndex: 1, direction: Down, want: 3},
		{name: "bottom right up to the tile closer to its center", rects: grid, fromIndex: 3, direction: Up, want: 1},
		{name: "bottom left up to the wide tile", rects: grid, fromIndex: 2, direction: Up, want: 0},
		{name: "bottom left right to the bottom right", rects: grid, fromIndex: 2, direction: Right, want: 3},
		{name: "skips a tile with no cells", rects: withEmptyTile, fromIndex: 0, direction: Down, want: 2},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := NearestTile(testCase.rects, testCase.fromIndex, testCase.direction); got != testCase.want {
				t.Errorf("expected tile %d, got %d", testCase.want, got)
			}
		})
	}
}
