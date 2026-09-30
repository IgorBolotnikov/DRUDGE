package cmd

import (
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

func TestPrintNext(t *testing.T) {
	migration := &task.Task{ID: "9c8d7e6f-dbe9-4316-8aba-8a67a8f01f8f", Title: "Add the migration", Status: task.StatusDone}
	repository := &task.Task{ID: "7e6d5c4b-dbe9-4316-8aba-8a67a8f01f8f", Title: "Wire the repository", Status: task.StatusDone}
	next := &task.Task{ID: "2b3c4d5e-dbe9-4316-8aba-8a67a8f01f8f", Title: "Pick the next task", Status: task.StatusTodo}
	storage := &task.Task{ID: "4f2a1b3c-dbe9-4316-8aba-8a67a8f01f8f", Title: "Store the blockers", Status: task.StatusTodo}

	cases := []struct {
		name string
		pick task.Pick
		want string
	}{
		{
			name: "a task whose blockers are done",
			pick: task.Pick{Task: next, Blockers: []task.Blocker{{ID: migration.ID, Task: migration}}},
			want: "2b3c4d5e  Pick the next task\n",
		},
		{
			name: "no todo tasks",
			want: "No task can be started. There are no todo tasks.\n",
		},
		{
			name: "one todo task that is blocked",
			pick: task.Pick{Blocked: []task.BlockedTask{{Task: next, Holding: []task.TaskID{storage.ID}}}},
			want: "No task can be started. The only todo task is blocked:\n" +
				"  2b3c4d5e  Pick the next task  blocked by 4f2a1b3c\n",
		},
		{
			name: "every todo task blocked",
			pick: task.Pick{Blocked: []task.BlockedTask{
				{Task: next, Holding: []task.TaskID{storage.ID}},
				{Task: storage, Holding: []task.TaskID{migration.ID, repository.ID}},
			}},
			want: "No task can be started. 2 todo tasks are all blocked:\n" +
				"  2b3c4d5e  Pick the next task  blocked by 4f2a1b3c\n" +
				"  4f2a1b3c  Store the blockers  blocked by 9c8d7e6f, 7e6d5c4b\n",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			log := common.NewLogger("", "")

			out := captureOutput(func() { printNext(log, testCase.pick) })

			if out != testCase.want {
				t.Errorf("expected:\n%s\ngot:\n%s", testCase.want, out)
			}
		})
	}
}
