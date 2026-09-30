package drudger

import (
	"reflect"
	"testing"
)

func TestDrudgerHealth(t *testing.T) {
	cases := []struct {
		name          string
		sandbox       SandboxHealth
		workspace     WorkspaceHealth
		agent         AgentHealth
		wantUnchecked bool
		wantOk        bool
		wantFaults    []HealthFault
	}{
		{name: "never looked at any part", wantUnchecked: true},
		{name: "every part is fine", sandbox: SandboxUsable, workspace: WorkspaceUsable, agent: AgentReady, wantOk: true},
		{
			name:       "the sandbox is not there",
			sandbox:    SandboxGone,
			workspace:  WorkspaceUsable,
			agent:      AgentReady,
			wantFaults: []HealthFault{{Part: SandboxPart, State: string(SandboxGone)}},
		},
		{
			name:  "the vendor turned away an agent in an unchecked Drudger",
			agent: AgentRefused,
			wantFaults: []HealthFault{
				{Part: SandboxPart, State: string(SandboxUnchecked)},
				{Part: WorkspacePart, State: string(WorkspaceUnchecked)},
				{Part: AgentPart, State: string(AgentRefused)},
			},
		},
		{
			name:       "a workspace value this build does not know",
			sandbox:    SandboxUsable,
			workspace:  "hand-edited",
			agent:      AgentReady,
			wantFaults: []HealthFault{{Part: WorkspacePart, State: "hand-edited"}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			entry := &Drudger{SandboxHealth: testCase.sandbox, WorkspaceHealth: testCase.workspace, AgentHealth: testCase.agent}

			got := entry.Health()
			if got.IsUnchecked != testCase.wantUnchecked {
				t.Errorf("expected unchecked %v, got %v", testCase.wantUnchecked, got.IsUnchecked)
			}
			if got.IsOk() != testCase.wantOk {
				t.Errorf("expected ok %v, got %v", testCase.wantOk, got.IsOk())
			}
			if !reflect.DeepEqual(got.Faults, testCase.wantFaults) {
				t.Errorf("expected faults %v, got %v", testCase.wantFaults, got.Faults)
			}
		})
	}
}
