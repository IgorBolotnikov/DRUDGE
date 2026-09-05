package runner

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

// Event types and subtypes an agent writes to its stream.
const (
	streamEventSystem = "system"
	streamEventResult = "result"

	streamSubtypeInit = "init"
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
	stream, err := os.Open(common.RunStreamPath(runDir))
	if errors.Is(err, fs.ErrNotExist) {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("could not open the event stream of run directory %s: %w", runDir, err)
	}
	defer stream.Close()

	sessionID, err := sessionIDFromStream(stream)
	if err != nil {
		return "", fmt.Errorf("could not read the event stream of run directory %s: %w", runDir, err)
	}
	return sessionID, nil
}

// sessionIDFromStream reads the session id off the first event that carries
// one.
//
// The agent writes this file while drudge reads it, so the last line can be
// half-written. A line that does not parse is skipped. A stream that cannot be
// read at all, or that holds a line past the size limit, is an error, so a
// broken read is never mistaken for an agent that has not started yet.
func sessionIDFromStream(stream io.Reader) (string, error) {
	scanner := bufio.NewScanner(stream)
	scanner.Buffer(make([]byte, 0, streamLineBufferSize), streamLineMaxSize)

	for scanner.Scan() {
		var event streamEvent
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			continue
		}
		if event.carriesSessionID() {
			return event.SessionID, nil
		}
	}
	return "", scanner.Err()
}
