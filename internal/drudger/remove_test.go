package drudger

import (
	"testing"

	"drudge/internal/common"
)

func TestDrudgerService_RemoveRun(t *testing.T) {
	cases := []struct {
		name string
		// stream is what the run directory of the task holds. An empty one
		// stands for a task no agent has been given.
		stream  string
		wantRun bool
	}{
		{
			name:    "a task an agent has worked on",
			stream:  initEvent,
			wantRun: true,
		},
		{
			name: "a task no agent has been given",
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			projectDir := setupProjectDir(t)
			taskToRemove := todoTask()

			runDir := common.RunDir(projectDir, string(taskToRemove.ID))
			if testCase.stream != "" {
				writeStream(t, runDir, testCase.stream)
			}

			service := newTestService(taskToRemove)

			hasRun, err := service.RemoveRun(taskToRemove.ID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if hasRun != testCase.wantRun {
				t.Errorf("expected a run directory to be reported: %v, got %v", testCase.wantRun, hasRun)
			}

			isLeft, err := common.Exists(runDir)
			if err != nil {
				t.Fatalf("could not check the run directory: %v", err)
			}
			if isLeft {
				t.Error("expected the run directory to be gone")
			}
		})
	}
}
