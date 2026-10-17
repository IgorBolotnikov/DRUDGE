package drudger

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
)

// Sample events of a real run, one per line of the stream.
const (
	initEvent      = `{"type":"system","subtype":"init","session_id":"ebe60e03-991c-44f9-861c-f9e779298552","cwd":"/home/igor/Dev/drudge","model":"claude-sonnet-5"}`
	assistantEvent = `{"type":"assistant","session_id":"ebe60e03-991c-44f9-861c-f9e779298552","message":{"content":[{"type":"text","text":"Working on it"}]}}`
	resultEvent    = `{"type":"result","subtype":"success","is_error":false,"num_turns":3,"duration_ms":8664,"total_cost_usd":0.0695,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","result":"Done"}`

	sampleSessionID = "ebe60e03-991c-44f9-861c-f9e779298552"
)

// Sample events of a run the vendor turned away.
const (
	authRefusedText  = "Failed to authenticate: OAuth session expired and could not be refreshed"
	rateLimitedText  = "Rate limit exceeded, try again later"
	vendorOutageText = "The service is overloaded"

	authRefusedEvent       = `{"type":"assistant","session_id":"ebe60e03-991c-44f9-861c-f9e779298552","message":{"content":[{"type":"text","text":"` + authRefusedText + `"}]},"error":"authentication_failed","is_api_error_message":true}`
	authRefusedResultEvent = `{"type":"result","subtype":"success","is_error":true,"num_turns":1,"duration_ms":1200,"total_cost_usd":0,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","terminal_reason":"api_error","result":"` + authRefusedText + `"}`

	rateLimitedEvent       = `{"type":"assistant","session_id":"ebe60e03-991c-44f9-861c-f9e779298552","message":{"content":[{"type":"text","text":"` + rateLimitedText + `"}]},"error":"rate_limit_error","is_api_error_message":true}`
	rateLimitedResultEvent = `{"type":"result","subtype":"success","is_error":true,"num_turns":1,"duration_ms":800,"total_cost_usd":0,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","terminal_reason":"api_error","result":"` + rateLimitedText + `"}`

	// A refusal the agent reported without an error code, which is a refusal
	// drudge cannot tell apart from any other.
	uncodedRefusalResultEvent = `{"type":"result","subtype":"success","is_error":true,"num_turns":1,"duration_ms":700,"total_cost_usd":0,"session_id":"ebe60e03-991c-44f9-861c-f9e779298552","terminal_reason":"api_error","result":"` + vendorOutageText + `"}`
)

// unreadableEvent is a line the fake stream reader fails on.
const unreadableEvent = "a line the stream reader cannot read"

// sampleEvent is what a stream reader reads off one sample event.
type sampleEvent struct {
	sessionID       string
	result          *SessionResult
	isVendorRefusal bool
	errorCode       string
}

// sampleEvents holds what every sample event says, keyed by its line.
var sampleEvents = map[string]sampleEvent{
	initEvent: {sessionID: sampleSessionID},
	resultEvent: {sessionID: sampleSessionID, result: &SessionResult{
		Subtype: "success", HasSucceeded: true, NumTurns: 3, Duration: 8664 * time.Millisecond, CostUSD: 0.0695, Text: "Done",
	}},
	erroredResultEvent: {sessionID: sampleSessionID, result: &SessionResult{
		IsError: true, Subtype: "success", HasSucceeded: true, NumTurns: 2, Duration: 4000 * time.Millisecond, CostUSD: 0.02, Text: "Could not build",
	}},
	cutOffResultEvent: {sessionID: sampleSessionID, result: &SessionResult{
		Subtype: "error_max_turns", NumTurns: 50, Duration: 900000 * time.Millisecond, CostUSD: 1.5, Text: "Ran out of turns",
	}},
	authRefusedEvent: {errorCode: "authentication_failed"},
	authRefusedResultEvent: {sessionID: sampleSessionID, isVendorRefusal: true, result: &SessionResult{
		IsError: true, Subtype: "success", HasSucceeded: true, NumTurns: 1, Duration: 1200 * time.Millisecond, Text: authRefusedText,
	}},
	rateLimitedEvent: {errorCode: "rate_limit_error"},
	rateLimitedResultEvent: {sessionID: sampleSessionID, isVendorRefusal: true, result: &SessionResult{
		IsError: true, Subtype: "success", HasSucceeded: true, NumTurns: 1, Duration: 800 * time.Millisecond, Text: rateLimitedText,
	}},
	uncodedRefusalResultEvent: {sessionID: sampleSessionID, isVendorRefusal: true, result: &SessionResult{
		IsError: true, Subtype: "success", HasSucceeded: true, NumTurns: 1, Duration: 700 * time.Millisecond, Text: vendorOutageText,
	}},
}

func TestReadSessionID(t *testing.T) {
	cases := []struct {
		name string
		// lines are written to the stream file, unless they are nil, which
		// stands for a run that has no stream file yet.
		lines []string
		want  string
	}{
		{name: "no stream file yet"},
		{name: "an empty stream file", lines: []string{}},
		{name: "a stream holding an init event", lines: []string{initEvent}, want: sampleSessionID},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runs := newFakeRunRepo()
			taskID := todoTask().ID
			if testCase.lines != nil {
				runs.writeStream(taskID, testCase.lines...)
			}

			got, err := testRunReader(runs).sessionID(taskID)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != testCase.want {
				t.Errorf("expected session id %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestReadSessionID_UnreadableStream(t *testing.T) {
	// A stream that cannot be read is not a missing stream, so the read has to
	// say so instead of answering with an empty id.
	runs := newFakeRunRepo()
	runs.streamErr = errors.New("is a directory")

	if _, err := testRunReader(runs).sessionID(todoTask().ID); err == nil {
		t.Fatal("expected an error for a stream that cannot be read")
	}
}

func TestRunReader_HarnessWithoutAStreamReader(t *testing.T) {
	cases := []struct {
		name       string
		configured Harness
		// recorded is the harness the run recorded, empty for a run that
		// recorded none.
		recorded Harness
	}{
		{name: "a recorded harness", configured: HarnessClaudeCode, recorded: HarnessOpencode},
		{name: "the configured harness of a run that recorded none", configured: HarnessOpencode},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runs := newFakeRunRepo()
			taskID := todoTask().ID
			runs.writeStream(taskID, initEvent)
			if testCase.recorded != "" {
				runs.write(taskID, common.RunHarnessName, string(testCase.recorded))
			}
			reader := testRunReader(runs)
			reader.configured = testCase.configured

			_, err := reader.sessionID(taskID)
			if err == nil {
				t.Fatal("expected an error for a harness with no stream reader")
			}
			if !strings.Contains(err.Error(), string(HarnessOpencode)) {
				t.Errorf("expected the error to name harness %s, got %q", HarnessOpencode, err)
			}
		})
	}
}

// joinLines renders event lines as the agent writes them, one per line.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
