package drudger

import (
	"fmt"
	"slices"
	"strings"

	"github.com/IgorBolotnikov/DRUDGE/internal/task"
)

// StreamReader reads the event stream one harness writes.
type StreamReader interface {
	// Decode turns one line of the stream into the Steps it holds. An event a
	// harness writes only for its own bookkeeping, like a thinking block or a
	// heartbeat, decodes to no Steps.
	Decode(line string) []Step
	// SessionID returns the agent's session id. It is empty while the agent
	// has not written the event carrying it.
	SessionID(stream []byte) (string, error)
	// Outcome reads what the end of the stream says about the run.
	Outcome(stream []byte) (StreamOutcome, error)
}

// StreamOutcome is what the end of an event stream says about a run.
type StreamOutcome struct {
	// Result is what the agent reported on the terminal event of its run, nil
	// until it gets there. Its vendor error class is left empty.
	Result *SessionResult
	// IsVendorRefusal tells whether the terminal event says that the vendor
	// turned the run away.
	IsVendorRefusal bool
	// ErrorCode is the last failure code the agent reported on a turn, empty
	// when it reported none.
	ErrorCode string
}

// runReader reads the files of a run. It answers questions about the event
// stream with the stream reader of the harness that launched the run.
type runReader struct {
	RunRepository
	streams map[Harness]StreamReader
	// configured is the harness of a run that recorded none.
	configured Harness
}

func (service *DrudgerService) runReader() runReader {
	return runReader{RunRepository: service.runs, streams: service.streams, configured: service.settings.Harness}
}

// harnessOf returns the harness a run was launched with, or the configured
// harness for a run that recorded none.
func (reader runReader) harnessOf(taskID task.TaskID) (Harness, error) {
	harness, isRecorded, err := reader.ReadHarness(taskID)
	if err != nil {
		return "", fmt.Errorf("could not read the harness of the run of task %s: %w", taskID, err)
	}
	if !isRecorded {
		return reader.configured, nil
	}
	return harness, nil
}

// streamReaderOf returns the stream reader of the harness of a run. It
// refuses a harness DRUDGE has no stream reader for.
func (reader runReader) streamReaderOf(taskID task.TaskID) (StreamReader, error) {
	harness, err := reader.harnessOf(taskID)
	if err != nil {
		return nil, err
	}
	streamReader, ok := reader.streams[harness]
	if !ok {
		return nil, fmt.Errorf("DRUDGE cannot read the event stream of harness %q that ran task %s, it reads %s", harness, taskID, readableHarnesses(reader.streams))
	}
	return streamReader, nil
}

// sessionID picks the agent's session id out of the event stream of a run.
// An empty id means the agent has not written the event carrying it yet,
// which is the normal state right after a launch.
func (reader runReader) sessionID(taskID task.TaskID) (string, error) {
	return readStream(reader, taskID, StreamReader.SessionID)
}

// outcome picks what the end of a run says out of its event stream.
func (reader runReader) outcome(taskID task.TaskID) (StreamOutcome, error) {
	return readStream(reader, taskID, StreamReader.Outcome)
}

// streamHasContent reports whether the agent has written anything at all to
// its event stream.
func streamHasContent(runs RunRepository, taskID task.TaskID) (bool, error) {
	content, _, err := runs.ReadStream(taskID)
	if err != nil {
		return false, fmt.Errorf("could not read the event stream of task %s: %w", taskID, err)
	}
	return len(content) > 0, nil
}

// readStream hands the event stream of a run to a stream reader. A run with no
// stream yet gets the zero value, since an agent that has not written anything
// is the normal state right after a launch.
func readStream[T any](reader runReader, taskID task.TaskID, read func(StreamReader, []byte) (T, error)) (T, error) {
	var zero T

	streamReader, err := reader.streamReaderOf(taskID)
	if err != nil {
		return zero, err
	}

	content, isPresent, err := reader.ReadStream(taskID)
	if err != nil {
		return zero, fmt.Errorf("could not read the event stream of task %s: %w", taskID, err)
	}
	if !isPresent {
		return zero, nil
	}

	value, err := read(streamReader, content)
	if err != nil {
		return zero, fmt.Errorf("could not read the event stream of task %s: %w", taskID, err)
	}
	return value, nil
}

// refuseUnreadableHarness refuses to launch an agent on the configured harness
// when DRUDGE cannot read the event stream it would write.
func (service *DrudgerService) refuseUnreadableHarness() error {
	harness := service.settings.Harness
	if _, ok := service.streams[harness]; ok {
		return nil
	}
	return fmt.Errorf("DRUDGE cannot read the event stream of harness %q, check the Drudger settings in the config, it reads %s", harness, readableHarnesses(service.streams))
}

// readableHarnesses lists the harnesses with a stream reader, sorted and
// quoted.
func readableHarnesses(streams map[Harness]StreamReader) string {
	names := make([]string, 0, len(streams))
	for harness := range streams {
		names = append(names, fmt.Sprintf("%q", harness))
	}
	slices.Sort(names)
	return strings.Join(names, ", ")
}
