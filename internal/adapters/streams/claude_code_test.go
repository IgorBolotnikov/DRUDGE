package streams

import (
	"reflect"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
)

// Lines trimmed from real runs under .drudge/runs, used to build the table of
// TestClaudeCodeDecoder_Decode.
const (
	initLine = `{"type":"system","subtype":"init","session_id":"13b19929-4c9d-4eca-80dd-4270c085ab69","cwd":"/home/igorbolotnikov/Dev/drudge/.drudge/worktrees/slot-1","model":"claude-opus-5-5"}`

	agentTextLine = `{"type":"assistant","message":{"content":[{"type":"text","text":"Now build, test, and format check per project conventions."}]}}`
	thinkingLine  = `{"type":"assistant","message":{"content":[{"type":"thinking","thinking":"","signature":"opaque"}]}}`

	bashLine  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"git show HEAD~2 | cat | head -1200"}}]}}`
	readLine  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Read","input":{"file_path":"/home/igorbolotnikov/Dev/drudge/.drudge/worktrees/slot-1/internal/adapters/persistence/task_repo.go"}}]}}`
	editLine  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Edit","input":{"file_path":"/home/igorbolotnikov/Dev/drudge/.drudge/worktrees/slot-1/internal/cmd/task.go","old_string":"a","new_string":"b"}}]}}`
	writeLine = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Write","input":{"file_path":"/home/igorbolotnikov/Dev/drudge/DONE.md","content":"Time: 2026-09-06 20:36:51 UTC"}}]}}`
	grepLine  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Grep","input":{"pattern":"RunDescribed","path":"internal"}}]}}`
	globLine  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Glob","input":{"pattern":"internal/**/*.go"}}]}}`
	skillLine = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Skill","input":{"skill":"impl-ticket","args":"TICT-1234"}}]}}`
	taskLine  = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Task","input":{"description":"Find runlog related files","subagent_type":"Explore"}}]}}`
	agentLine = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Agent","input":{"description":"Explore runlog and drudger code","subagent_type":"Explore"}}]}}`

	unknownToolLine = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"WebSearch","input":{"query":"golang json"}}]}}`

	multiLineCommandLine = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"go build ./...\ngo test ./..."}}]}}`

	// summaryPastCapLine holds a 222-character Bash command, trimmed from a
	// real run, that the summary cap cuts to its first summaryCapLength
	// characters.
	summaryPastCapLine = `{"type":"assistant","message":{"content":[{"type":"tool_use","name":"Bash","input":{"command":"grep -rl \"TICT-1234\" /home/igorbolotnikov/Dev/drudge/.backlog 2>/dev/null; echo \"---all tickets---\"; find /home/igorbolotnikov/Dev/drudge/.backlog -type f -iname \"*.md\" | xargs grep -l \"status.*ready-for-agent\" 2>/dev/null"}}]}}`

	thinkingTokensLine   = `{"type":"system","subtype":"thinking_tokens","estimated_tokens":50,"session_id":"b25f91fe-bb2d-4590-8b72-409837297490"}`
	rateLimitLine        = `{"type":"rate_limit_event","rate_limit_info":{"status":"allowed"},"session_id":"b25f91fe-bb2d-4590-8b72-409837297490"}`
	taskStartedLine      = `{"type":"system","subtype":"task_started","task_id":"bqkmkt0p3","tool_use_id":"toolu_01Er92AJBWSJZzwnhggA4gn6"}`
	taskNotificationLine = `{"type":"system","subtype":"task_notification","task_id":"bqkmkt0p3","status":"completed"}`
	taskProgressLine     = `{"type":"system","subtype":"task_progress","task_id":"ad1b648afe640a64b"}`
	commandsChangedLine  = `{"type":"system","subtype":"commands_changed","commands":[]}`
	vcsStateChangedLine  = `{"type":"system","subtype":"vcs_state_changed","kind":"merge"}`

	successfulToolResultLine = `{"type":"user","message":{"content":[{"type":"tool_result","content":"Launching skill: code-style","tool_use_id":"toolu_013ZzTMxvWS8RfWWQasjycQ6"}]}}`
	failedToolResultLine     = `{"type":"user","message":{"content":[{"type":"tool_result","content":"Exit code 123\n---all tickets---","is_error":true,"tool_use_id":"toolu_01U9ZWRVbhiY5UbrnGAsFCSb"}]}}`

	successfulResultLine = `{"type":"result","subtype":"success","is_error":false,"num_turns":12,"duration_ms":43071,"total_cost_usd":0.3134686,"session_id":"67410977-f80b-49e2-9976-6f84cbc3644f","result":"Done."}`
	failedResultLine     = `{"type":"result","subtype":"success","is_error":true,"num_turns":5,"duration_ms":125000,"total_cost_usd":0.5,"session_id":"67410977-f80b-49e2-9976-6f84cbc3644f","result":"Could not finish."}`

	malformedLine   = `{"type":"assistant","message":{"content":[{"type":"text","text":"Half`
	unknownTypeLine = `{"type":"tool_progress","tool_use_id":"toolu_01JjYGKtngbGAEtZwiJKegB9-heartbeat-0","tool_name":"Bash"}`
)

func TestClaudeCodeDecoder_Decode(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []drudger.Step
	}{
		{
			name: "an init event",
			line: initLine,
			want: []drudger.Step{{Kind: drudger.StepInit, Summary: "session 13b19929… claude-opus-5-5 · /home/igorbolotnikov/Dev/drudge/.drudge/worktrees/slot-1", Raw: initLine}},
		},
		{
			name: "agent text",
			line: agentTextLine,
			want: []drudger.Step{{Kind: drudger.StepAgentText, Summary: "Now build, test, and format check per project conventions.", Raw: agentTextLine}},
		},
		{
			name: "thinking",
			line: thinkingLine,
			want: nil,
		},
		{
			name: "a Bash call",
			line: bashLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "git show HEAD~2 | cat | head -1200", Raw: bashLine}},
		},
		{
			name: "a Read call",
			line: readLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Read", Summary: "/home/igorbolotnikov/Dev/drudge/.drudge/worktrees/slot-1/internal/adapters/persistence/task_repo.go", Raw: readLine}},
		},
		{
			name: "an Edit call shows only its path",
			line: editLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Edit", Summary: "/home/igorbolotnikov/Dev/drudge/.drudge/worktrees/slot-1/internal/cmd/task.go", Raw: editLine}},
		},
		{
			name: "a Write call shows only its path",
			line: writeLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Write", Summary: "/home/igorbolotnikov/Dev/drudge/DONE.md", Raw: writeLine}},
		},
		{
			name: "a Grep call",
			line: grepLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Grep", Summary: "RunDescribed", Raw: grepLine}},
		},
		{
			name: "a Glob call",
			line: globLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Glob", Summary: "internal/**/*.go", Raw: globLine}},
		},
		{
			name: "a Skill call",
			line: skillLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Skill", Summary: "impl-ticket", Raw: skillLine}},
		},
		{
			name: "a Task call",
			line: taskLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Task", Summary: "Find runlog related files", Raw: taskLine}},
		},
		{
			name: "an Agent call",
			line: agentLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Agent", Summary: "Explore runlog and drudger code", Raw: agentLine}},
		},
		{
			name: "an unknown tool shows its name only",
			line: unknownToolLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "WebSearch", Summary: "", Raw: unknownToolLine}},
		},
		{
			name: "a multi-line command keeps only its first line",
			line: multiLineCommandLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Bash", Summary: "go build ./..." + summaryCutSuffix, Raw: multiLineCommandLine}},
		},
		{
			name: "a summary past the cap is cut to it",
			line: summaryPastCapLine,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "Bash", Summary: `grep -rl "TICT-1234" /home/igorbolotnikov/Dev/drudge/.backlog 2>/dev/null; echo "---all tickets---"; find /home/igorbolo` + summaryCutSuffix, Raw: summaryPastCapLine}},
		},
		{name: "thinking_tokens is noise", line: thinkingTokensLine, want: nil},
		{name: "rate_limit_event is noise", line: rateLimitLine, want: nil},
		{name: "task_started is noise", line: taskStartedLine, want: nil},
		{name: "task_notification is noise", line: taskNotificationLine, want: nil},
		{name: "task_progress is noise", line: taskProgressLine, want: nil},
		{name: "commands_changed is noise", line: commandsChangedLine, want: nil},
		{name: "vcs_state_changed is noise", line: vcsStateChangedLine, want: nil},
		{
			name: "a successful tool result is hidden",
			line: successfulToolResultLine,
			want: nil,
		},
		{
			name: "a failed tool result",
			line: failedToolResultLine,
			want: []drudger.Step{{Kind: drudger.StepToolError, Summary: "Exit code 123" + summaryCutSuffix, Raw: failedToolResultLine}},
		},
		{
			name: "a successful final result",
			line: successfulResultLine,
			want: []drudger.Step{{Kind: drudger.StepFinalResult, Summary: "got shit done · 12 turns · 43s · $0.31", Raw: successfulResultLine}},
		},
		{
			name: "a failed final result",
			line: failedResultLine,
			want: []drudger.Step{{Kind: drudger.StepFinalResult, Summary: "fucked up · 5 turns · 2m5s · $0.50", Raw: failedResultLine}},
		},
		{
			name: "a malformed line is printed as it is",
			line: malformedLine,
			want: []drudger.Step{{Kind: drudger.StepUnrecognized, Raw: malformedLine}},
		},
		{
			name: "an unknown event type is printed as it is",
			line: unknownTypeLine,
			want: []drudger.Step{{Kind: drudger.StepUnrecognized, Raw: unknownTypeLine}},
		},
	}

	decoder := NewClaudeCodeDecoder()
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := decoder.Decode(testCase.line)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("expected %+v, got %+v", testCase.want, got)
			}
		})
	}
}
