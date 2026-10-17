package streams

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
)

// Sample events of a real run, one per line of the stream.
const (
	runInitEvent      = `{"type":"system","subtype":"init","session_id":"ebe60e03-991c-44f9-861c-f9e779298552","cwd":"/home/igor/Dev/drudge","model":"claude-sonnet-5"}`
	runAssistantEvent = `{"type":"assistant","session_id":"ebe60e03-991c-44f9-861c-f9e779298552","message":{"content":[{"type":"text","text":"Working on it"}]}}`
	runResultEvent    = `{"type":"result","subtype":"success","is_error":false,"num_turns":3,"duration_ms":8664,"total_cost_usd":0.0695,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","result":"Done"}`

	runErroredResultEvent = `{"type":"result","subtype":"success","is_error":true,"num_turns":2,"duration_ms":4000,"total_cost_usd":0.02,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","result":"Could not build"}`
	runCutOffResultEvent  = `{"type":"result","subtype":"error_max_turns","is_error":false,"num_turns":50,"duration_ms":900000,"total_cost_usd":1.5,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","result":"Ran out of turns"}`

	sampleSessionID = "ebe60e03-991c-44f9-861c-f9e779298552"
)

// Sample events of a run the vendor turned away.
const (
	authRefusedText = "Failed to authenticate: OAuth session expired and could not be refreshed"
	outageText      = "The service is overloaded"

	authRefusedEvent       = `{"type":"assistant","session_id":"ebe60e03-991c-44f9-861c-f9e779298552","message":{"content":[{"type":"text","text":"` + authRefusedText + `"}]},"error":"authentication_failed","is_api_error_message":true}`
	authRefusedResultEvent = `{"type":"result","subtype":"success","is_error":true,"num_turns":1,"duration_ms":1200,"total_cost_usd":0,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","terminal_reason":"api_error","result":"` + authRefusedText + `"}`

	// A refusal the agent reported without an error code.
	uncodedRefusalResultEvent = `{"type":"result","subtype":"success","is_error":true,"num_turns":1,"duration_ms":700,"total_cost_usd":0,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","terminal_reason":"api_error","result":"` + outageText + `"}`
)

func TestClaudeCodeReader_SessionID(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{name: "an init event", lines: []string{runInitEvent}, want: sampleSessionID},
		{name: "a whole finished run", lines: []string{runInitEvent, runAssistantEvent, runResultEvent}, want: sampleSessionID},
		{name: "a result event alone", lines: []string{runResultEvent}, want: sampleSessionID},
		{name: "an empty stream"},
		{name: "a first line that is not written yet", lines: []string{""}},
		{name: "a half-written init event", lines: []string{`{"type":"system","subtype":"init","session`}},
		{name: "a line that is not JSON at all", lines: []string{"Starting claude agent"}},
		{name: "an init event after a broken line", lines: []string{"not json", runInitEvent}, want: sampleSessionID},
		{name: "an event type drudge does not know", lines: []string{`{"type":"rate_limit_event","session_id":"other"}`}},
		{name: "an assistant event alone", lines: []string{runAssistantEvent}},
		{name: "an init event without a session id", lines: []string{`{"type":"system","subtype":"init"}`}},
		{name: "a system event that is not the init one", lines: []string{`{"type":"system","subtype":"compact","session_id":"other"}`}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := NewClaudeCodeReader().SessionID(joinLines(testCase.lines))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != testCase.want {
				t.Errorf("expected session id %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestClaudeCodeReader_Outcome(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  drudger.StreamOutcome
	}{
		{name: "an empty stream"},
		{name: "a run still going", lines: []string{runInitEvent, runAssistantEvent}},
		{name: "a half-written result event", lines: []string{runInitEvent, `{"type":"result","subtype":"succ`}},
		{
			name:  "a run that finished the work",
			lines: []string{runInitEvent, runAssistantEvent, runResultEvent},
			want: drudger.StreamOutcome{Result: &drudger.SessionResult{
				Subtype: "success", HasSucceeded: true, NumTurns: 3, Duration: 8664 * time.Millisecond, CostUSD: 0.0695, Text: "Done",
			}},
		},
		{
			name:  "a result event flagged as an error",
			lines: []string{runInitEvent, runErroredResultEvent},
			want: drudger.StreamOutcome{Result: &drudger.SessionResult{
				IsError: true, Subtype: "success", HasSucceeded: true, NumTurns: 2, Duration: 4000 * time.Millisecond, CostUSD: 0.02, Text: "Could not build",
			}},
		},
		{
			name:  "a result event that did not succeed",
			lines: []string{runInitEvent, runCutOffResultEvent},
			want: drudger.StreamOutcome{Result: &drudger.SessionResult{
				Subtype: "error_max_turns", NumTurns: 50, Duration: 900000 * time.Millisecond, CostUSD: 1.5, Text: "Ran out of turns",
			}},
		},
		{
			name:  "a run the vendor refused with an error code",
			lines: []string{runInitEvent, authRefusedEvent, authRefusedResultEvent},
			want: drudger.StreamOutcome{
				Result: &drudger.SessionResult{
					IsError: true, Subtype: "success", HasSucceeded: true, NumTurns: 1, Duration: 1200 * time.Millisecond, Text: authRefusedText,
				},
				IsVendorRefusal: true,
				ErrorCode:       "authentication_failed",
			},
		},
		{
			name:  "a run the vendor refused without an error code",
			lines: []string{runInitEvent, uncodedRefusalResultEvent},
			want: drudger.StreamOutcome{
				Result: &drudger.SessionResult{
					IsError: true, Subtype: "success", HasSucceeded: true, NumTurns: 1, Duration: 700 * time.Millisecond, Text: outageText,
				},
				IsVendorRefusal: true,
			},
		},
		{
			name:  "an error code on a run still going",
			lines: []string{runInitEvent, authRefusedEvent},
			want:  drudger.StreamOutcome{ErrorCode: "authentication_failed"},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := NewClaudeCodeReader().Outcome(joinLines(testCase.lines))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("expected outcome %+v, got %+v", testCase.want, got)
			}
		})
	}
}

func TestClaudeCodeReader_LinePastTheSizeLimit(t *testing.T) {
	oversized := []byte(strings.Repeat("x", streamLineMaxSize+1))
	reader := NewClaudeCodeReader()

	if _, err := reader.SessionID(oversized); err == nil {
		t.Error("expected the session id read to fail on a line past the size limit")
	}
	if _, err := reader.Outcome(oversized); err == nil {
		t.Error("expected the outcome read to fail on a line past the size limit")
	}
}

// joinLines renders event lines as the agent writes them, one per line.
func joinLines(lines []string) []byte {
	if len(lines) == 0 {
		return nil
	}
	return []byte(strings.Join(lines, "\n") + "\n")
}
