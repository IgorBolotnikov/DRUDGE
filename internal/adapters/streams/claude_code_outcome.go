package streams

import (
	"bufio"
	"bytes"
	"encoding/json"
	"time"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
)

// TODO: this whole file has to be revisited after a live testing

// terminalReasonAPIError is what a run ends on when the vendor turned the
// agent away before it could work.
const terminalReasonAPIError = "api_error"

// A single event holds whole tool arguments and whole tool results, so a line
// can run far past the default scanner limit.
const (
	streamLineBufferSize = 64 * 1024
	streamLineMaxSize    = 4 * 1024 * 1024
)

// SessionID reads the session id off the first event that carries one. The
// agent puts the id on the init event it writes first, and repeats it on the
// terminal result event.
func (*ClaudeCodeReader) SessionID(stream []byte) (string, error) {
	var sessionID string

	err := scanStream(stream, func(parsed streamLine) bool {
		if !carriesSessionID(parsed) {
			return true
		}
		sessionID = parsed.SessionID
		return false
	})
	return sessionID, err
}

func carriesSessionID(parsed streamLine) bool {
	if parsed.SessionID == "" {
		return false
	}
	if parsed.Type == eventTypeResult {
		return true
	}
	return parsed.Type == eventTypeSystem && parsed.Subtype == subtypeInit
}

// Outcome reads the terminal result event and the last error code the agent
// reported.
func (*ClaudeCodeReader) Outcome(stream []byte) (drudger.StreamOutcome, error) {
	var outcome drudger.StreamOutcome

	err := scanStream(stream, func(parsed streamLine) bool {
		if parsed.Error != "" {
			outcome.ErrorCode = parsed.Error
		}
		if parsed.Type == eventTypeResult {
			outcome.Result = sessionResultOf(parsed)
			outcome.IsVendorRefusal = parsed.TerminalReason == terminalReasonAPIError
		}
		return true
	})
	return outcome, err
}

func sessionResultOf(terminal streamLine) *drudger.SessionResult {
	return &drudger.SessionResult{
		IsError:      terminal.IsError,
		Subtype:      terminal.Subtype,
		HasSucceeded: terminal.Subtype == resultSubtypeSuccess,
		NumTurns:     terminal.NumTurns,
		Duration:     time.Duration(terminal.DurationMS) * time.Millisecond,
		CostUSD:      terminal.TotalCostUSD,
		Text:         terminal.Result,
	}
}

// scanStream hands every event of a stream to visit, until visit returns false
// to say it has what it came for.
//
// The agent writes the stream while drudge reads it, so the last line can be
// half-written. A line that does not parse is skipped. A line past the size
// limit is an error, so a broken read is never mistaken for an agent that has
// not started yet.
func scanStream(stream []byte, visit func(streamLine) bool) error {
	scanner := bufio.NewScanner(bytes.NewReader(stream))
	scanner.Buffer(make([]byte, 0, streamLineBufferSize), streamLineMaxSize)

	for scanner.Scan() {
		var parsed streamLine
		if err := json.Unmarshal(scanner.Bytes(), &parsed); err != nil {
			continue
		}
		if !visit(parsed) {
			return nil
		}
	}
	return scanner.Err()
}
