package cmd

import (
	"strings"
	"testing"
)

func TestListLines(t *testing.T) {
	cases := []struct {
		name    string
		columns []column
		rows    [][]string
		want    []string
	}{
		{
			name:    "counts the rows and rules each column",
			columns: []column{{Title: "SLUG", Width: 6}, {Title: "NAME"}},
			rows:    [][]string{{"drudge", "Drudge"}},
			want: []string{
				"Things (1):",
				"  SLUG    NAME",
				"  ------  ----",
				"  drudge  Drudge",
			},
		},
		{
			name:    "cuts a value too long for its column",
			columns: []column{{Title: "TITLE", Width: 10}, {Title: "TICKET"}},
			rows:    [][]string{{"a title nobody can fit", "R-002"}},
			want: []string{
				"Things (1):",
				"  TITLE       TICKET",
				"  ----------  ------",
				"  a title...  R-002",
			},
		},
		{
			name:    "leaves the last column whole",
			columns: []column{{Title: "SLOT", Width: 4}, {Title: "SANDBOX"}},
			rows:    [][]string{{"1", "a sandbox name far wider than its header"}},
			want: []string{
				"Things (1):",
				"  SLOT  SANDBOX",
				"  ----  -------",
				"  1     a sandbox name far wider than its header",
			},
		},
		{
			name:    "blanks a column the row says nothing about",
			columns: []column{{Title: "SLUG", Width: 6}, {Title: "NAME"}},
			rows:    [][]string{{"drudge"}},
			want: []string{
				"Things (1):",
				"  SLUG    NAME",
				"  ------  ----",
				"  drudge",
			},
		},
		{
			name:    "prints a percent sign as it stands",
			columns: []column{{Title: "TITLE"}},
			rows:    [][]string{{"cut CPU by 50%"}},
			want: []string{
				"Things (1):",
				"  TITLE",
				"  -----",
				"  cut CPU by 50%",
			},
		},
		{
			name:    "counts a value in characters, not bytes",
			columns: []column{{Title: "NAME", Width: 4}, {Title: "SLUG"}},
			rows:    [][]string{{"añçá", "done"}},
			want: []string{
				"Things (1):",
				"  NAME  SLUG",
				"  ----  ----",
				"  añçá  done",
			},
		},
		{
			name:    "no rows leaves the header standing",
			columns: []column{{Title: "SLUG", Width: 6}, {Title: "NAME"}},
			rows:    nil,
			want: []string{
				"Things (0):",
				"  SLUG    NAME",
				"  ------  ----",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := listLines("Things", testCase.columns, testCase.rows)

			if len(got) != len(testCase.want) {
				t.Fatalf("expected %d lines, got %d:\n%s", len(testCase.want), len(got), strings.Join(got, "\n"))
			}
			for index := range testCase.want {
				if got[index] != testCase.want[index] {
					t.Errorf("line %d: expected %q, got %q", index, testCase.want[index], got[index])
				}
			}
		})
	}
}
