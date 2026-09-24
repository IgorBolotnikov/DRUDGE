package cmd

import (
	"strings"
	"testing"
)

func TestCLIRun_PrintsHelp(t *testing.T) {
	cases := []struct {
		name string
		args []string
	}{
		{name: "no arguments", args: nil},
		{name: "the help flag", args: []string{helpFlag}},
		{name: "the short help flag", args: []string{helpFlagShort}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())
			cli := NewCLI("v1.2.3")
			cli.Register(TaskCmd, ProjectCmd, DrudgerCmd)

			var err error
			output := captureOutput(func() { err = cli.Run(testCase.args) })

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			drudgerAt := strings.Index(output, "  drudger")
			projectAt := strings.Index(output, "  project")
			taskAt := strings.Index(output, "  task")
			if drudgerAt == -1 || projectAt == -1 || taskAt == -1 {
				t.Fatalf("expected every command in the help, got:\n%s", output)
			}
			if drudgerAt > projectAt || projectAt > taskAt {
				t.Errorf("expected the commands in alphabetical order, got:\n%s", output)
			}
			if !strings.HasPrefix(output, "Available commands:\n") {
				t.Errorf("expected the help to skip the logo outside a terminal, got:\n%s", output)
			}
		})
	}
}

func TestCLIRun_RefusesAnUnknownCommand(t *testing.T) {
	cli := NewCLI("v1.2.3")
	cli.Register(TaskCmd)

	err := cli.Run([]string{"deploy"})

	if err == nil || !strings.Contains(err.Error(), "unknown command: deploy") {
		t.Errorf("expected an unknown command error, got %v", err)
	}
}

func TestCLIRun_PrintsTheVersion(t *testing.T) {
	cli := NewCLI("v1.2.3")

	var err error
	output := captureOutput(func() { err = cli.Run([]string{versionFlag}) })

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if output != "drg v1.2.3\n" {
		t.Errorf("expected %q, got %q", "drg v1.2.3\n", output)
	}
}
