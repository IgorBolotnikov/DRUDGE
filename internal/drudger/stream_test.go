package drudger

import (
	"errors"
	"strings"
	"testing"
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

func TestSessionIDFromStream(t *testing.T) {
	cases := []struct {
		name  string
		lines []string
		want  string
	}{
		{name: "an init event", lines: []string{initEvent}, want: sampleSessionID},
		{name: "a whole finished run", lines: []string{initEvent, assistantEvent, resultEvent}, want: sampleSessionID},
		{name: "a result event alone", lines: []string{resultEvent}, want: sampleSessionID},
		{name: "an empty stream"},
		{name: "a first line that is not written yet", lines: []string{""}},
		{name: "a half-written init event", lines: []string{`{"type":"system","subtype":"init","session`}},
		{name: "a line that is not JSON at all", lines: []string{"Starting claude agent"}},
		{name: "an init event after a broken line", lines: []string{"not json", initEvent}, want: sampleSessionID},
		{name: "an event type drudge does not know", lines: []string{`{"type":"rate_limit_event","session_id":"other"}`}},
		{name: "an init event without a session id", lines: []string{`{"type":"system","subtype":"init"}`}},
		{name: "a system event that is not the init one", lines: []string{`{"type":"system","subtype":"compact","session_id":"other"}`}},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got, err := sessionIDFromStream(strings.NewReader(joinLines(testCase.lines)))
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != testCase.want {
				t.Errorf("expected session id %q, got %q", testCase.want, got)
			}
		})
	}
}

func TestSessionIDFromStream_LinePastTheSizeLimit(t *testing.T) {
	oversized := strings.Repeat("x", streamLineMaxSize+1)

	if _, err := sessionIDFromStream(strings.NewReader(oversized)); err == nil {
		t.Fatal("expected an error for a line past the size limit")
	}
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

			got, err := readSessionID(runs, taskID)
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

	if _, err := readSessionID(runs, todoTask().ID); err == nil {
		t.Fatal("expected an error for a stream that cannot be read")
	}
}

// joinLines renders event lines as the agent writes them, one per line.
func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}
