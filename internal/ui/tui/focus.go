package tui

// Direction is a way focus moves on the screen.
type Direction int

// Directions focus moves in.
const (
	Left Direction = iota
	Down
	Up
	Right
)

// neighbor is a Tile that lies in a direction from the focused Tile and how
// near it is.
type neighbor struct {
	// isAlongside reports whether the Tile shares rows with the focused Tile
	// when the direction is left or right, or columns when it is up or down.
	isAlongside bool
	// gap is how many cells lie between the two Tiles in the direction.
	gap int
	// centerDistance is how far apart the centers of the two Tiles are across
	// the direction, in half cells.
	centerDistance int
}

// isNearer reports whether the neighbor is a better target for focus than
// other. A Tile alongside wins first, then the smaller gap, then the closer
// center.
func (candidate neighbor) isNearer(other neighbor) bool {
	if candidate.isAlongside != other.isAlongside {
		return candidate.isAlongside
	}
	if candidate.gap != other.gap {
		return candidate.gap < other.gap
	}
	return candidate.centerDistance < other.centerDistance
}

// NearestTile returns the index of the Rect nearest to the Rect at fromIndex
// in a direction. Only a Rect that lies wholly on that side and takes cells
// counts. It returns fromIndex when no Rect does.
func NearestTile(rects []Rect, fromIndex int, direction Direction) int {
	from := rects[fromIndex]
	nearestIndex := fromIndex
	var nearest neighbor
	for index, rect := range rects {
		if index == fromIndex || rect.isEmpty() {
			continue
		}
		candidate, ok := neighborOf(from, rect, direction)
		if !ok {
			continue
		}
		if nearestIndex == fromIndex || candidate.isNearer(nearest) {
			nearestIndex = index
			nearest = candidate
		}
	}
	return nearestIndex
}

// neighborOf measures rect from the focused Rect. ok is false when rect does
// not lie wholly in the direction.
func neighborOf(from, rect Rect, direction Direction) (candidate neighbor, ok bool) {
	switch direction {
	case Left:
		if rect.right() > from.X {
			return neighbor{}, false
		}
		return acrossRows(from, rect, from.X-rect.right()), true
	case Right:
		if rect.X < from.right() {
			return neighbor{}, false
		}
		return acrossRows(from, rect, rect.X-from.right()), true
	case Up:
		if rect.bottom() > from.Y {
			return neighbor{}, false
		}
		return acrossColumns(from, rect, from.Y-rect.bottom()), true
	case Down:
		if rect.Y < from.bottom() {
			return neighbor{}, false
		}
		return acrossColumns(from, rect, rect.Y-from.bottom()), true
	}
	return neighbor{}, false
}

// acrossRows measures a neighbor to the left or the right.
func acrossRows(from, rect Rect, gap int) neighbor {
	return neighbor{
		isAlongside:    overlaps(from.Y, from.bottom(), rect.Y, rect.bottom()),
		gap:            gap,
		centerDistance: halfCellDistance(from.Y, from.Height, rect.Y, rect.Height),
	}
}

// acrossColumns measures a neighbor above or below.
func acrossColumns(from, rect Rect, gap int) neighbor {
	return neighbor{
		isAlongside:    overlaps(from.X, from.right(), rect.X, rect.right()),
		gap:            gap,
		centerDistance: halfCellDistance(from.X, from.Width, rect.X, rect.Width),
	}
}

// overlaps reports whether two spans of cells share a cell. Each span starts
// at its first cell and ends after its last.
func overlaps(firstStart, firstEnd, secondStart, secondEnd int) bool {
	return max(firstStart, secondStart) < min(firstEnd, secondEnd)
}

// halfCellDistance returns how far apart the centers of two spans are, in
// half cells, which keeps the centers of odd spans whole.
func halfCellDistance(firstStart, firstSize, secondStart, secondSize int) int {
	firstCenter := 2*firstStart + firstSize
	secondCenter := 2*secondStart + secondSize
	if firstCenter > secondCenter {
		return firstCenter - secondCenter
	}
	return secondCenter - firstCenter
}
