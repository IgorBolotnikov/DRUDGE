package tui

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"charm.land/lipgloss/v2"
)

// Node is one node of the layout of a Tab: a Split or a Tile. A Node fills
// exactly the size it last got.
type Node interface {
	SetSize(width, height int)
	Render() string
	// Tiles returns the Tiles of the Node in tree order: depth first, left to
	// right and top to bottom.
	Tiles() []*Tile
	// Weight is the share of the space of its Split the Node takes, relative to
	// the other children of the Split.
	Weight() float64
}

// Arrangement is how a Split places its children.
type Arrangement string

// Arrangements of a Split.
const (
	SideBySide Arrangement = "side-by-side"
	Stacked    Arrangement = "stacked"
)

// defaultWeight is the weight of a Node that takes an even share of its Split.
const defaultWeight = 1

// minSplitChildren is the fewest children a Split holds. A Split with one
// child is that child.
const minSplitChildren = 2

// Split places its children side by side or stacked and divides its space
// between them by weight.
type Split struct {
	arrangement Arrangement
	weight      float64
	children    []Node
	childSizes  []int
}

// NewSplit returns a Split of children. NewTab checks the Split.
func NewSplit(arrangement Arrangement, weight float64, children ...Node) *Split {
	return &Split{arrangement: arrangement, weight: weight, children: children}
}

func (split *Split) Weight() float64 {
	return split.weight
}

func (split *Split) Tiles() []*Tile {
	var tiles []*Tile
	for _, child := range split.children {
		tiles = append(tiles, child.Tiles()...)
	}
	return tiles
}

// SetSize divides the width between side by side children and the height
// between stacked children.
func (split *Split) SetSize(width, height int) {
	weights := make([]float64, 0, len(split.children))
	for _, child := range split.children {
		weights = append(weights, child.Weight())
	}
	if split.arrangement == SideBySide {
		split.childSizes = divide(width, weights)
		for index, child := range split.children {
			child.SetSize(split.childSizes[index], height)
		}
		return
	}
	split.childSizes = divide(height, weights)
	for index, child := range split.children {
		child.SetSize(width, split.childSizes[index])
	}
}

// Render joins the lines of the children. A stacked child with no height
// adds no line.
func (split *Split) Render() string {
	renderedChildren := make([]string, 0, len(split.children))
	for index, child := range split.children {
		// A stacked child with no rows renders as an empty string. The join
		// below still adds a line for it, which makes the Split one line
		// taller than its height, so the child is left out. A side by side
		// child with no columns keeps the line count and needs no skip.
		if split.arrangement == Stacked && split.childSizes[index] == 0 {
			continue
		}
		renderedChildren = append(renderedChildren, child.Render())
	}
	if split.arrangement == SideBySide {
		return lipgloss.JoinHorizontal(lipgloss.Top, renderedChildren...)
	}
	return strings.Join(renderedChildren, "\n")
}

// divide splits size cells between weights. Each part ends where its share of
// the weights so far ends, rounded to a whole cell, so the parts add up to
// size.
func divide(size int, weights []float64) []int {
	totalWeight := 0.0
	for _, weight := range weights {
		totalWeight += weight
	}
	parts := make([]int, len(weights))
	weightSoFar := 0.0
	start := 0
	for index, weight := range weights {
		weightSoFar += weight
		// The part ends at the cell that the weights up to and including this
		// part reach: their fraction of the total weight times the size,
		// rounded to a whole cell. Three weights of 1 over 41 cells end at
		// 13.7 → 14, 27.3 → 27 and 41, so the parts are 14, 13 and 14.
		end := int(math.Round(float64(size) * weightSoFar / totalWeight))
		parts[index] = end - start
		start = end
	}
	return parts
}

// validateNode refuses a layout the Tab cannot divide: a missing Node, a
// weight that is not above 0, an unknown arrangement or a Split with fewer
// than two children.
func validateNode(node Node) error {
	if node == nil {
		return errors.New("the layout has an empty node, put a split or a tile there")
	}
	if weight := node.Weight(); weight <= 0 {
		return fmt.Errorf("weight %v is not above 0, use a positive weight", weight)
	}
	split, isSplit := node.(*Split)
	if !isSplit {
		return nil
	}
	if split.arrangement != SideBySide && split.arrangement != Stacked {
		return fmt.Errorf("unknown arrangement %q, use %q or %q", split.arrangement, SideBySide, Stacked)
	}
	if len(split.children) < minSplitChildren {
		return fmt.Errorf("a split holds %d tiles and needs at least %d, put its tile in place of the split", len(split.children), minSplitChildren)
	}
	for _, child := range split.children {
		if err := validateNode(child); err != nil {
			return err
		}
	}
	return nil
}
