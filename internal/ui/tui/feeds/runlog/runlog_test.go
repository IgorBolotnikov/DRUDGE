package runlog

import (
	"errors"
	"reflect"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/common"
	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const (
	testProjectSlug             = "uploader"
	shownTaskID     task.TaskID = "3f9a1c2e-0000-0000-0000-000000000001"
)

var errDiskOnFire = errors.New("disk on fire")

// fakeRunSource returns its reads of the run in order. A read with an error
// fails with it.
type fakeRunSource struct {
	reads     []fakeRead
	readCount int
}

type fakeRead struct {
	read *drudger.RunStepsRead
	err  error
}

func (source *fakeRunSource) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return nil, nil
}

func (source *fakeRunSource) ListDrudgers(projectSlug string, page int, size int) (common.Page[*drudger.Drudger], error) {
	return common.Page[*drudger.Drudger]{}, nil
}

func (source *fakeRunSource) RecordFinishedSessions(projectSlug string) ([]task.TaskID, error) {
	return nil, nil
}

func (source *fakeRunSource) RunStepsFrom(projectSlug string, requestedID task.TaskID, cursor drudger.RunCursor) (*drudger.RunStepsRead, error) {
	next := source.reads[source.readCount]
	source.readCount++
	return next.read, next.err
}

func stepRead(summaries ...string) *drudger.RunStepsRead {
	read := &drudger.RunStepsRead{Task: &task.Task{ID: shownTaskID}, HasRun: true, IsDecoded: true}
	for _, summary := range summaries {
		read.Steps = append(read.Steps, drudger.Step{Kind: drudger.StepAgentText, Summary: summary})
	}
	return read
}

func TestKind_Run(t *testing.T) {
	cases := []struct {
		name         string
		reads        []fakeRead
		wantMessages []views.RunStepsReadMsg
		wantErrs     []error
		// wantIsCursor holds whether the state after each run is a cursor. A
		// state that is no cursor is the nil state of a feed that never read.
		wantIsCursor []bool
	}{
		{
			name:  "each run sends the steps of its read",
			reads: []fakeRead{{read: stepRead("Reading the uploader")}, {read: stepRead("Adding the retry", "Running the tests")}},
			wantMessages: []views.RunStepsReadMsg{
				{TaskID: shownTaskID, IsFirstRead: true, Read: stepRead("Reading the uploader")},
				{TaskID: shownTaskID, Read: stepRead("Adding the retry", "Running the tests")},
			},
			wantErrs:     []error{nil, nil},
			wantIsCursor: []bool{true, true},
		},
		{
			name:  "a failed first read keeps the feed at its first read",
			reads: []fakeRead{{err: errDiskOnFire}, {read: stepRead("Reading the uploader")}},
			wantMessages: []views.RunStepsReadMsg{
				{TaskID: shownTaskID, IsFirstRead: true, Err: errDiskOnFire},
				{TaskID: shownTaskID, IsFirstRead: true, Read: stepRead("Reading the uploader")},
			},
			wantErrs:     []error{errDiskOnFire, nil},
			wantIsCursor: []bool{false, true},
		},
		{
			name:  "a failed later read keeps the cursor",
			reads: []fakeRead{{read: stepRead("Reading the uploader")}, {err: errDiskOnFire}, {read: stepRead("Adding the retry")}},
			wantMessages: []views.RunStepsReadMsg{
				{TaskID: shownTaskID, IsFirstRead: true, Read: stepRead("Reading the uploader")},
				{TaskID: shownTaskID, Err: errDiskOnFire},
				{TaskID: shownTaskID, Read: stepRead("Adding the retry")},
			},
			wantErrs:     []error{nil, errDiskOnFire, nil},
			wantIsCursor: []bool{true, true, true},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			source := &fakeRunSource{reads: testCase.reads}
			sources := feeds.Sources{Drudgers: source}
			feed := views.Feed{Name: views.RunLogFeed, TaskID: shownTaskID}
			var state any
			for index := range testCase.reads {
				msg, next, err := Kind.Run(sources, testProjectSlug, feed, state)
				if !errors.Is(err, testCase.wantErrs[index]) {
					t.Errorf("run %d: expected the error %v, got %v", index+1, testCase.wantErrs[index], err)
				}
				if !reflect.DeepEqual(msg, testCase.wantMessages[index]) {
					t.Errorf("run %d: expected the message %+v, got %+v", index+1, testCase.wantMessages[index], msg)
				}
				if _, isCursor := next.(drudger.RunCursor); isCursor != testCase.wantIsCursor[index] {
					t.Errorf("run %d: expected the state to be a cursor %t, got %#v", index+1, testCase.wantIsCursor[index], next)
				}
				state = next
			}
		})
	}
}
