package streams

import (
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
)

// Event types the opencode harness writes with its JSON format.
const (
	opencodeEventStepFinish = "step_finish"
	opencodeEventText       = "text"
	opencodeEventError      = "error"
)

// opencodeReasonStop is the reason of the step that ends a run the agent
// finished.
const opencodeReasonStop = "stop"

// OpencodeReader reads the JSON event stream the opencode harness writes.
type OpencodeReader struct{}

// NewOpencodeReader builds a reader for the JSON format the opencode harness
// writes.
func NewOpencodeReader() *OpencodeReader {
	return &OpencodeReader{}
}

// opencodeLine is one line of an opencode event stream. Only the fields drudge
// reads are decoded.
type opencodeLine struct {
	Type string `json:"type"`
	// Timestamp is in milliseconds.
	Timestamp int64  `json:"timestamp"`
	SessionID string `json:"sessionID"`
	Part      struct {
		Reason string  `json:"reason"`
		Cost   float64 `json:"cost"`
		Text   string  `json:"text"`
	} `json:"part"`
	Error struct {
		Name string `json:"name"`
		Data struct {
			Message string `json:"message"`
		} `json:"data"`
	} `json:"error"`
}

// Decode turns every line into one unrecognized Step holding the line as it
// is.
func (*OpencodeReader) Decode(line string) []drudger.Step {
	return unrecognizedStep(line)
}

// SessionID reads the session id off the first event that carries one. Every
// event of the stream carries it.
func (*OpencodeReader) SessionID(stream []byte) (string, error) {
	var sessionID string

	err := scanStream(stream, func(parsed opencodeLine) bool {
		sessionID = parsed.SessionID
		return sessionID == ""
	})
	return sessionID, err
}

// Outcome adds up the steps of a run. The stream has no terminal event, so the
// result exists once a step ends with the stop reason or an error event
// appears.
func (*OpencodeReader) Outcome(stream []byte) (drudger.StreamOutcome, error) {
	var (
		result         drudger.SessionResult
		hasEnded       bool
		hasTimestamp   bool
		firstTimestamp int64
		lastTimestamp  int64
		lastReason     string
		lastText       string
		lastError      string
		lastErrorName  string
	)

	err := scanStream(stream, func(parsed opencodeLine) bool {
		if !hasTimestamp {
			firstTimestamp = parsed.Timestamp
			hasTimestamp = true
		}
		lastTimestamp = parsed.Timestamp

		switch parsed.Type {
		case opencodeEventStepFinish:
			result.NumTurns++
			result.CostUSD += parsed.Part.Cost
			lastReason = parsed.Part.Reason
			result.HasSucceeded = lastReason == opencodeReasonStop
			if result.HasSucceeded {
				hasEnded = true
			}
		case opencodeEventText:
			lastText = parsed.Part.Text
		case opencodeEventError:
			result.IsError = true
			result.HasSucceeded = false
			lastErrorName = parsed.Error.Name
			lastError = parsed.Error.Data.Message
			hasEnded = true
		}
		return true
	})
	if err != nil || !hasEnded {
		return drudger.StreamOutcome{}, err
	}

	result.Subtype = lastReason
	result.Text = lastText
	if result.IsError {
		result.Subtype = lastErrorName
		result.Text = lastError
	}
	result.Duration = time.Duration(lastTimestamp-firstTimestamp) * time.Millisecond
	return drudger.StreamOutcome{Result: &result}, nil
}
