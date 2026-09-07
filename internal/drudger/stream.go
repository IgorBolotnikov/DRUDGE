package drudger

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"

	"drudge/internal/common"
)

// TODO: this whole file has to be revisited after a live testing

// Event types and subtypes an agent writes to its stream.
const (
	streamEventSystem = "system"
	streamEventResult = "result"

	streamSubtypeInit = "init"

	// streamSubtypeSuccess is the only result subtype that means the agent
	// finished the work it was given.
	streamSubtypeSuccess = "success"

	// terminalReasonAPIError is what a run ends on when the vendor turned the
	// agent away before it could work.
	terminalReasonAPIError = "api_error"
)

// A single event holds whole tool arguments and whole tool results, so a line
// can run far past the default scanner limit.
const (
	streamLineBufferSize = 64 * 1024
	streamLineMaxSize    = 4 * 1024 * 1024
)

// streamEvent is one line of an agent's event stream. The harness writes many
// more fields than these. Only the ones drudge reads are decoded.
type streamEvent struct {
	Type      string `json:"type"`
	Subtype   string `json:"subtype"`
	SessionID string `json:"session_id"`

	// The rest is written on the terminal result event only.
	IsError      bool    `json:"is_error"`
	NumTurns     int     `json:"num_turns"`
	DurationMS   int64   `json:"duration_ms"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Result       string  `json:"result"`

	// TerminalReason says what ended the run. It is the only field that names a
	// vendor-level failure.
	TerminalReason string `json:"terminal_reason"`

	// Error is the code of a failure the agent hit on a turn.
	Error string `json:"error"`
}

// vendorRefused reports whether a terminal event says that the vendor turned
// the run away.
func (event streamEvent) vendorRefused() bool {
	return event.Type == streamEventResult && event.TerminalReason == terminalReasonAPIError
}

// carriesSessionID tells whether an event names the agent's session. The agent
// puts the id on the init event it writes first, and repeats it on the
// terminal result event.
func (event streamEvent) carriesSessionID() bool {
	if event.SessionID == "" {
		return false
	}
	if event.Type == streamEventResult {
		return true
	}
	return event.Type == streamEventSystem && event.Subtype == streamSubtypeInit
}

// readSessionID picks the agent's session id out of the event stream of a run
// directory. An empty id means the agent has not written the event carrying it
// yet, which is the normal state right after a launch.
func readSessionID(runDir string) (string, error) {
	return readStream(runDir, sessionIDFromStream)
}

// readOutcome picks what the end of a run says out of the event stream of a
// run directory.
func readOutcome(runDir string) (streamOutcome, error) {
	return readStream(runDir, outcomeFromStream)
}

// streamHasContent reports whether the agent has written anything at all to
// its event stream.
func streamHasContent(runDir string) (bool, error) {
	stream, err := os.Stat(common.RunStreamPath(runDir))
	if errors.Is(err, fs.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("could not check the event stream of run directory %s: %w", runDir, err)
	}
	return stream.Size() > 0, nil
}

// readStream opens the event stream of a run directory and hands it to a
// reader. A run directory with no stream file yet gets the zero value, since
// an agent that has not written anything is the normal state right after a
// launch.
func readStream[T any](runDir string, read func(io.Reader) (T, error)) (T, error) {
	var zero T

	stream, err := os.Open(common.RunStreamPath(runDir))
	if errors.Is(err, fs.ErrNotExist) {
		return zero, nil
	}
	if err != nil {
		return zero, fmt.Errorf("could not open the event stream of run directory %s: %w", runDir, err)
	}
	defer stream.Close()

	value, err := read(stream)
	if err != nil {
		return zero, fmt.Errorf("could not read the event stream of run directory %s: %w", runDir, err)
	}
	return value, nil
}

// sessionIDFromStream reads the session id off the first event that carries
// one.
func sessionIDFromStream(stream io.Reader) (string, error) {
	var sessionID string

	err := scanStream(stream, func(event streamEvent) bool {
		if !event.carriesSessionID() {
			return true
		}
		sessionID = event.SessionID
		return false
	})
	return sessionID, err
}

// streamOutcome is what the end of an event stream says about a run.
type streamOutcome struct {
	// terminal is the result event the agent writes at the very end of a run.
	// An unfinished run tesults in nil.
	terminal *streamEvent
	// errorCode is the last failure code the agent reported on a turn, empty
	// when it reported none.
	errorCode string
}

// outcomeFromStream reads the terminal result event and the last error code
// the agent reported.
func outcomeFromStream(stream io.Reader) (streamOutcome, error) {
	var outcome streamOutcome

	err := scanStream(stream, func(event streamEvent) bool {
		if event.Error != "" {
			outcome.errorCode = event.Error
		}
		if event.Type == streamEventResult {
			terminal := event
			outcome.terminal = &terminal
		}
		return true
	})
	return outcome, err
}

// scanStream hands every event of a stream to visit, until visit returns false
// to say it has what it came for.
//
// The agent writes this file while drudge reads it, so the last line can be
// half-written. A line that does not parse is skipped. A stream that cannot be
// read at all, or that holds a line past the size limit, is an error, so a
// broken read is never mistaken for an agent that has not started yet.
func scanStream(stream io.Reader, visit func(streamEvent) bool) error {
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, streamLineBufferSize), streamLineMaxSize)

	for scanner.Scan() {
		var event streamEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if !visit(event) {
			return nil
		}
	}
	return scanner.Err()
}
