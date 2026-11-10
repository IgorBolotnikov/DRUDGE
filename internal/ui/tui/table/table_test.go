package table

import (
	"slices"
	"testing"
)

// testColumns are a required status, a fill title and three columns that drop
// in the order ticket, parent, blockers.
var testColumns = []Column{
	{Title: "STATUS", IsRequired: true},
	{Title: "TITLE", MinWidth: 5, IsFill: true},
	{Title: "BLOCKERS", Priority: 3},
	{Title: "TICKET", Priority: 1},
	{Title: "PARENT", Priority: 2},
}

var testRows = [][]string{
	{"in-progress", "Add retry to uploader", "3f9a1c2e", "UP-12", "5b20d7aa"},
	{"done", "Write the backoff", "", "", ""},
}

func TestNewLayout_Titles(t *testing.T) {
	cases := []struct {
		name  string
		width int
		want  []string
	}{
		{name: "every column fits", width: 80, want: []string{"STATUS", "TITLE", "BLOCKERS", "TICKET", "PARENT"}},
		{name: "exact width of every column", width: 46, want: []string{"STATUS", "TITLE", "BLOCKERS", "TICKET", "PARENT"}},
		{name: "lowest priority drops first", width: 45, want: []string{"STATUS", "TITLE", "BLOCKERS", "PARENT"}},
		{name: "next priority drops next", width: 37, want: []string{"STATUS", "TITLE", "BLOCKERS"}},
		{name: "only required columns left", width: 27, want: []string{"STATUS", "TITLE"}},
		{name: "required columns stay when they do not fit", width: 5, want: []string{"STATUS", "TITLE"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := NewLayout(testColumns, testRows, testCase.width).Titles()
			if !slices.Equal(got, testCase.want) {
				t.Errorf("expected columns %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestLayout_Row(t *testing.T) {
	cases := []struct {
		name  string
		width int
		row   []string
		want  string
	}{
		{name: "fill column takes the width left", width: 58, row: testRows[1], want: "done         Write the backoff                            "},
		{name: "fill column is cut with an ellipsis", width: 35, row: testRows[0], want: "in-progress  Add retry t…  3f9a1c2e"},
		{name: "fill column keeps its minimum width", width: 10, row: testRows[0], want: "in-progress  Add …"},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := NewLayout(testColumns, testRows, testCase.width).Row(testCase.row); got != testCase.want {
				t.Errorf("expected %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestLayout_Width(t *testing.T) {
	cases := []struct {
		name  string
		width int
		want  int
	}{
		{name: "fills the width", width: 80, want: 80},
		{name: "fills the width after dropping columns", width: 40, want: 40},
		{name: "wider than the width when the required columns do not fit", width: 5, want: 18},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := NewLayout(testColumns, testRows, testCase.width).Width(); got != testCase.want {
				t.Errorf("expected width %d, got %d", testCase.want, got)
			}
		})
	}
}
