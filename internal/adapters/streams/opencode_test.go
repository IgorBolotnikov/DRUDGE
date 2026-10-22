package streams

import (
	"reflect"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
)

// Sample events of real opencode runs, one per line of the stream.
const (
	opencodeStepStartEvent   = `{"type":"step_start","timestamp":1792691372442,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"id":"prt_164bb8192001YDrwiuVo4VFRSi","messageID":"msg_164bb47f1001Nov8ezXBdAGdJm","sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","type":"step-start"}}`
	opencodeToolUseEvent     = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"bash","callID":"Qx4YIRU8KcgAk4nNHYeuywXpe8EBlkGi","state":{"status":"completed","input":{"command":"ls"},"output":"a.txt\nerr.log\nout.jsonl\n","metadata":{"output":"a.txt\nerr.log\nout.jsonl\n","exit":0,"truncated":false},"title":"ls","time":{"start":1792691374928,"end":1792691374934}},"id":"prt_164bb89f50018RRvrOThLicdjM","sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","messageID":"msg_164bb47f1001Nov8ezXBdAGdJm"}}`
	opencodeToolStepEndEvent = `{"type":"step_finish","timestamp":1792691374990,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"id":"prt_164bb8b66001SCoww8DuM2FDKV","reason":"tool-calls","messageID":"msg_164bb47f1001Nov8ezXBdAGdJm","sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","type":"step-finish","tokens":{"total":11671,"input":11547,"output":124,"reasoning":0,"cache":{"write":0,"read":0}},"cost":0.004609125}}`
	opencodeTextEvent        = `{"type":"text","timestamp":1792691376218,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"id":"prt_164bb9048001dU8udVr2PU8D1g","messageID":"msg_164bb8b87001X6PSzvDnFQoIl2","sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","type":"text","text":"hello","time":{"start":1792691376200,"end":1792691376204}}}`
	opencodeStopStepEndEvent = `{"type":"step_finish","timestamp":1792691376219,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"id":"prt_164bb904e0014xRaSUS4RXK6kp","reason":"stop","messageID":"msg_164bb8b87001X6PSzvDnFQoIl2","sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","type":"step-finish","tokens":{"total":11780,"input":77,"output":31,"reasoning":0,"cache":{"write":0,"read":11672}},"cost":0.000098625}}`
	opencodeErrorEvent       = `{"type":"error","timestamp":1792691381364,"sessionID":"ses_e9b445d02ffeWYRRGzwTctfG4Y","error":{"name":"UnknownError","data":{"message":"Unexpected server error. Check server logs for details.","ref":"err_f903945e"}}}`

	opencodeUnreachableEvent = `{"type":"error","timestamp":1792691450884,"sessionID":"ses_e9b444620ffeoxYhCaOwZEJuHV","error":{"name":"APIError","data":{"message":"Cannot connect to API: Unable to connect. Is the computer able to access the url?","isRetryable":true,"metadata":{"url":"http://127.0.0.1:9/v1/chat/completions"}}}}`

	opencodeSessionID      = "ses_e9b44b965ffeO2DLNgYKWU4JQ3"
	opencodeErrorSessionID = "ses_e9b445d02ffeWYRRGzwTctfG4Y"
)

func TestOpencodeReader_SessionID(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{name: "a first event", lines: []string{opencodeStepStartEvent}, want: opencodeSessionID},
		{name: "a whole finished run", lines: []string{opencodeStepStartEvent, opencodeToolUseEvent, opencodeToolStepEndEvent, opencodeTextEvent, opencodeStopStepEndEvent}, want: opencodeSessionID},
		{name: "an error event alone", lines: []string{opencodeErrorEvent}, want: opencodeErrorSessionID},
		{name: "the first event wins", lines: []string{opencodeStepStartEvent, opencodeErrorEvent}, want: opencodeSessionID},
		{name: "an empty stream"},
		{name: "a half-written first event", lines: []string{`{"type":"step_start","timestamp":1792691372442,"sessi`}},
		{name: "a line that is not JSON at all", lines: []string{"Starting opencode"}},
		{name: "an event after a broken line", lines: []string{"not json", opencodeStepStartEvent}, want: opencodeSessionID},
		{name: "an event without a session id before one with it", lines: []string{`{"type":"step_start"}`, opencodeTextEvent}, want: opencodeSessionID},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := NewOpencodeReader().SessionID(joinLines(testCase.lines))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != testCase.want {
				t.Errorf("expected session id %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestOpencodeReader_Outcome(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  drudger.StreamOutcome
	}{
		{name: "an empty stream"},
		{name: "a run still going", lines: []string{opencodeStepStartEvent, opencodeToolUseEvent, opencodeToolStepEndEvent}},
		{name: "a half-written last step", lines: []string{opencodeStepStartEvent, opencodeTextEvent, `{"type":"step_finish","timestamp":1792691376219,"part":{"reason":"st`}},
		{
			name:  "a run that finished the work",
			lines: []string{opencodeStepStartEvent, opencodeToolUseEvent, opencodeToolStepEndEvent, opencodeTextEvent, opencodeStopStepEndEvent},
			want: drudger.StreamOutcome{Result: &drudger.SessionResult{
				Subtype: "stop", HasSucceeded: true, NumTurns: 2, Duration: 3777 * time.Millisecond, CostUSD: 0.004609125 + 0.000098625, Text: "hello",
			}},
		},
		{
			name:  "a run that hit an error",
			lines: []string{opencodeErrorEvent},
			want: drudger.StreamOutcome{Result: &drudger.SessionResult{
				IsError: true, Subtype: "UnknownError", Text: "Unexpected server error. Check server logs for details.",
			}},
		},
		{
			name:  "an error after the work was done",
			lines: []string{opencodeStepStartEvent, opencodeTextEvent, opencodeStopStepEndEvent, opencodeErrorEvent},
			want: drudger.StreamOutcome{Result: &drudger.SessionResult{
				IsError: true, Subtype: "UnknownError", NumTurns: 1, Duration: 8922 * time.Millisecond, CostUSD: 0.000098625, Text: "Unexpected server error. Check server logs for details.",
			}},
		},
		{
			name:  "a provider that could not be reached",
			lines: []string{opencodeUnreachableEvent},
			want: drudger.StreamOutcome{
				Result: &drudger.SessionResult{
					IsError: true, Subtype: "APIError", Text: "Cannot connect to API: Unable to connect. Is the computer able to access the url?",
				},
				IsVendorRefusal: true,
				ErrorCode:       "api_error",
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := NewOpencodeReader().Outcome(joinLines(testCase.lines))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("expected outcome %+v, got %+v", testCase.want, got)
			}
		})
	}
}

func TestOpencodeReader_Outcome_providerErrors(t *testing.T) {
	cases := []struct {
		name          string
		errorJSON     string
		wantIsRefusal bool
		wantErrorCode string
	}{
		{name: "a key that could not be loaded", errorJSON: `{"name":"ProviderAuthError","data":{"providerID":"anthropic","message":"no key"}}`, wantIsRefusal: true, wantErrorCode: "authentication_error"},
		{name: "an unauthorized request", errorJSON: `{"name":"APIError","data":{"message":"unauthorized","statusCode":401}}`, wantIsRefusal: true, wantErrorCode: "authentication_error"},
		{name: "a forbidden request", errorJSON: `{"name":"APIError","data":{"message":"forbidden","statusCode":403}}`, wantIsRefusal: true, wantErrorCode: "authentication_error"},
		{name: "a rate limit", errorJSON: `{"name":"APIError","data":{"message":"slow down","statusCode":429}}`, wantIsRefusal: true, wantErrorCode: "rate_limit_error"},
		{name: "an unavailable provider", errorJSON: `{"name":"APIError","data":{"message":"unavailable","statusCode":503}}`, wantIsRefusal: true, wantErrorCode: "overloaded_error"},
		{name: "an overloaded provider", errorJSON: `{"name":"APIError","data":{"message":"overloaded","statusCode":529}}`, wantIsRefusal: true, wantErrorCode: "overloaded_error"},
		{name: "another status", errorJSON: `{"name":"APIError","data":{"message":"bad gateway","statusCode":502}}`, wantIsRefusal: true, wantErrorCode: "api_error"},
		{name: "a connection failure", errorJSON: `{"name":"APIError","data":{"message":"Cannot connect to API","isRetryable":true}}`, wantIsRefusal: true, wantErrorCode: "api_error"},
		{name: "an unknown error", errorJSON: `{"name":"UnknownError","data":{"message":"boom"}}`},
		{name: "an output too long", errorJSON: `{"name":"MessageOutputLengthError","data":{}}`},
		{name: "an aborted message", errorJSON: `{"name":"MessageAbortedError","data":{"message":"aborted"}}`},
		{name: "a context overflow", errorJSON: `{"name":"ContextOverflowError","data":{"message":"too long"}}`},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			line := `{"type":"error","timestamp":1792691450884,"sessionID":"ses_e9b444620ffeoxYhCaOwZEJuHV","error":` + testCase.errorJSON + `}`
			got, err := NewOpencodeReader().Outcome(joinLines([]string{line}))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Result == nil || !got.Result.IsError {
				t.Fatalf("expected an errored result, got %+v", got.Result)
			}
			if got.IsVendorRefusal != testCase.wantIsRefusal {
				t.Errorf("expected vendor refusal %t, got %t", testCase.wantIsRefusal, got.IsVendorRefusal)
			}
			if got.ErrorCode != testCase.wantErrorCode {
				t.Errorf("expected error code %q, got %q", testCase.wantErrorCode, got.ErrorCode)
			}
		})
	}
}

// Tool call lines in the shape of a real opencode run, used to build the table
// of TestOpencodeReader_Decode.
const (
	opencodeReadEvent  = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"read","callID":"call_read","state":{"status":"completed","input":{"filePath":"/workspace/internal/cmd/task.go"},"output":"package cmd"}}}`
	opencodeEditEvent  = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"edit","callID":"call_edit","state":{"status":"completed","input":{"filePath":"/workspace/internal/cmd/task.go","oldString":"a","newString":"b"},"output":""}}}`
	opencodeWriteEvent = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"write","callID":"call_write","state":{"status":"completed","input":{"filePath":"/workspace/DONE.md","content":"done"},"output":""}}}`
	opencodeGrepEvent  = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"grep","callID":"call_grep","state":{"status":"completed","input":{"pattern":"StreamReader","path":"internal"},"output":""}}}`
	opencodeGlobEvent  = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"glob","callID":"call_glob","state":{"status":"completed","input":{"pattern":"internal/**/*.go"},"output":""}}}`
	opencodeSkillEvent = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"skill","callID":"call_skill","state":{"status":"completed","input":{"name":"code-style"},"output":""}}}`
	opencodeTaskEvent  = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"task","callID":"call_task","state":{"status":"completed","input":{"description":"Find runlog files","prompt":"Find them","subagent_type":"explore"},"output":""}}}`

	opencodeUnknownToolEvent   = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"webfetch","callID":"call_webfetch","state":{"status":"completed","input":{"url":"https://opencode.ai"},"output":""}}}`
	opencodeMultiLineBashEvent = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"bash","callID":"call_bash","state":{"status":"completed","input":{"command":"go build ./...\ngo test ./..."},"output":""}}}`
	opencodeFailedToolUseEvent = `{"type":"tool_use","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"tool","tool":"read","callID":"call_failed","state":{"status":"error","input":{"filePath":"/workspace/missing.go"},"error":"File not found: /workspace/missing.go\nDid you mean missing_test.go?"}}}`
	opencodeUnknownEvent       = `{"type":"reasoning","timestamp":1792691374935,"sessionID":"ses_e9b44b965ffeO2DLNgYKWU4JQ3","part":{"type":"reasoning","text":"Thinking"}}`
	opencodeMalformedEvent     = `{"type":"text","timestamp":1792691376218,"part":{"text":"Hal`
)

func TestOpencodeReader_Decode(t *testing.T) {
	cases := []struct {
		name string
		line string
		want []drudger.Step
	}{
		{
			name: "agent text",
			line: opencodeTextEvent,
			want: []drudger.Step{{Kind: drudger.StepAgentText, Summary: "hello", Raw: opencodeTextEvent}},
		},
		{
			name: "a bash call",
			line: opencodeToolUseEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "bash", Summary: "ls", Raw: opencodeToolUseEvent}},
		},
		{
			name: "a read call",
			line: opencodeReadEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "read", Summary: "/workspace/internal/cmd/task.go", Raw: opencodeReadEvent}},
		},
		{
			name: "an edit call shows only its path",
			line: opencodeEditEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "edit", Summary: "/workspace/internal/cmd/task.go", Raw: opencodeEditEvent}},
		},
		{
			name: "a write call shows only its path",
			line: opencodeWriteEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "write", Summary: "/workspace/DONE.md", Raw: opencodeWriteEvent}},
		},
		{
			name: "a grep call",
			line: opencodeGrepEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "grep", Summary: "StreamReader", Raw: opencodeGrepEvent}},
		},
		{
			name: "a glob call",
			line: opencodeGlobEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "glob", Summary: "internal/**/*.go", Raw: opencodeGlobEvent}},
		},
		{
			name: "a skill call",
			line: opencodeSkillEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "skill", Summary: "code-style", Raw: opencodeSkillEvent}},
		},
		{
			name: "a task call",
			line: opencodeTaskEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "task", Summary: "Find runlog files", Raw: opencodeTaskEvent}},
		},
		{
			name: "an unknown tool shows its name alone",
			line: opencodeUnknownToolEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "webfetch", Raw: opencodeUnknownToolEvent}},
		},
		{
			name: "a multi-line command is cut at its first line",
			line: opencodeMultiLineBashEvent,
			want: []drudger.Step{{Kind: drudger.StepToolCall, Tool: "bash", Summary: "go build ./...…", Raw: opencodeMultiLineBashEvent}},
		},
		{
			name: "a failed tool call",
			line: opencodeFailedToolUseEvent,
			want: []drudger.Step{
				{Kind: drudger.StepToolCall, Tool: "read", Summary: "/workspace/missing.go", Raw: opencodeFailedToolUseEvent},
				{Kind: drudger.StepToolError, Summary: "File not found: /workspace/missing.go…", Raw: opencodeFailedToolUseEvent},
			},
		},
		{
			name: "a step start",
			line: opencodeStepStartEvent,
			want: nil,
		},
		{
			name: "a step finish",
			line: opencodeStopStepEndEvent,
			want: nil,
		},
		{
			name: "an error",
			line: opencodeErrorEvent,
			want: []drudger.Step{{Kind: drudger.StepFinalResult, Summary: "fucked up · UnknownError: Unexpected server error. Check server logs for details.", Raw: opencodeErrorEvent}},
		},
		{
			name: "an unknown event type",
			line: opencodeUnknownEvent,
			want: []drudger.Step{{Kind: drudger.StepUnrecognized, Raw: opencodeUnknownEvent}},
		},
		{
			name: "a half-written line",
			line: opencodeMalformedEvent,
			want: []drudger.Step{{Kind: drudger.StepUnrecognized, Raw: opencodeMalformedEvent}},
		},
		{
			name: "a line that is not JSON at all",
			line: "Starting opencode",
			want: []drudger.Step{{Kind: drudger.StepUnrecognized, Raw: "Starting opencode"}},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := NewOpencodeReader().Decode(testCase.line)
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("expected %+v, got %+v", testCase.want, got)
			}
		})
	}
}
