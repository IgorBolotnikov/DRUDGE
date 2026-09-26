package cmd

import (
	"strings"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestDescribeRemoval(t *testing.T) {
	removed := &task.Task{ID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Add the migration"}
	wireRepository := &task.Task{ID: "4f2a1b3c-dbe9-4316-8aba-8a67a8f01f8f", Title: "Wire the repository"}
	addEndpoint := &task.Task{ID: "7e6d5c4b-dbe9-4316-8aba-8a67a8f01f8f", Title: "Add the endpoint"}
	pickNext := &task.Task{ID: "2b3c4d5e-dbe9-4316-8aba-8a67a8f01f8f", Title: "Pick the next task"}

	cases := []struct {
		name       string
		dependents []*task.Task
		children   []*task.Task
		want       []string
	}{
		{
			name: "a task with no links",
			want: []string{`task 9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f "Add the migration"`},
		},
		{
			name:       "dependents",
			dependents: []*task.Task{wireRepository, addEndpoint},
			want: []string{
				`task 9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f "Add the migration"`,
				"2 tasks are blocked by it and will be unblocked:",
				"  4f2a1b3c  Wire the repository",
				"  7e6d5c4b  Add the endpoint",
			},
		},
		{
			name:     "one child",
			children: []*task.Task{pickNext},
			want: []string{
				`task 9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f "Add the migration"`,
				"1 task belongs to it and will be ungrouped:",
				"  2b3c4d5e  Pick the next task",
			},
		},
		{
			name:       "one dependent and children",
			dependents: []*task.Task{wireRepository},
			children:   []*task.Task{addEndpoint, pickNext},
			want: []string{
				`task 9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f "Add the migration"`,
				"1 task is blocked by it and will be unblocked:",
				"  4f2a1b3c  Wire the repository",
				"2 tasks belong to it and will be ungrouped:",
				"  7e6d5c4b  Add the endpoint",
				"  2b3c4d5e  Pick the next task",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			removal := task.Removal{Task: removed, Dependents: testCase.dependents, Children: testCase.children}

			got := describeRemoval(removal)
			if want := strings.Join(testCase.want, "\n"); got != want {
				t.Errorf("expected:\n%s\ngot:\n%s", want, got)
			}
		})
	}
}
