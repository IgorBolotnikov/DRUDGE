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

func TestOpencodeReader_DecodeLeavesEveryLineUnrecognized(t *testing.T) {
	lines := []string{opencodeStepStartEvent, opencodeTextEvent, opencodeErrorEvent, "not json"}

	for _, line := range lines {
		got := NewOpencodeReader().Decode(line)
		want := []drudger.Step{{Kind: drudger.StepUnrecognized, Raw: line}}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("expected %+v, got %+v", want, got)
		}
	}
}
