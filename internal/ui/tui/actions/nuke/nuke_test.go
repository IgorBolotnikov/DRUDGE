package nuke_test

import (
	"errors"
	"slices"
	"testing"

	"github.com/IgorBolotnikov/DRUDGE/internal/drudger"
	"github.com/IgorBolotnikov/DRUDGE/internal/task"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/actions/nuke"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

const testProjectSlug = "uploader"

const testTaskID task.TaskID = "1a2b3c4d-0000-4000-8000-000000000001"

var errDiskOnFire = errors.New("disk on fire")

// nukeCall is one nuke the fake service got.
type nukeCall struct {
	slot     int
	isForced bool
}

// fakeDrudgerService refuses every nuke that is not forced with nukeErr. It
// records every nuke.
type fakeDrudgerService struct {
	nukeErr error
	nukes   []nukeCall
}

func (service *fakeDrudgerService) RunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a nuke runs no task")
}

func (service *fakeDrudgerService) RerunTask(projectSlug string, requestedID task.TaskID, isDryRun bool) error {
	return errors.New("a nuke reruns no task")
}

func (service *fakeDrudgerService) MarkDone(projectSlug string, id task.TaskID) (*task.Task, []*task.Task, error) {
	return nil, nil, errors.New("a nuke marks no task done")
}

func (service *fakeDrudgerService) EditTask(projectSlug string, id task.TaskID, changes task.EditTaskDto) (*task.Task, error) {
	return nil, errors.New("a nuke edits no task")
}

func (service *fakeDrudgerService) EditTaskFromDraft(projectSlug string, id task.TaskID) (*task.Task, []string, error) {
	return nil, nil, errors.New("a nuke edits no task")
}

func (service *fakeDrudgerService) RetryPullRequests(projectSlug string, requestedID task.TaskID) (drudger.PullRequestsRetried, error) {
	return drudger.PullRequestsRetried{}, errors.New("a nuke opens no pull request")
}

func (service *fakeDrudgerService) ReclaimDrudgers(projectSlug string) ([]drudger.FreedSlot, error) {
	return nil, errors.New("a nuke reclaims no slot")
}

func (service *fakeDrudgerService) NukeDrudger(projectSlug string, slot int, isForced bool) error {
	if projectSlug != testProjectSlug {
		return errors.New("the nuke went to another project")
	}
	service.nukes = append(service.nukes, nukeCall{slot: slot, isForced: isForced})
	if isForced {
		return nil
	}
	return service.nukeErr
}

func (service *fakeDrudgerService) ReadDrudgers(projectSlug string) ([]*drudger.Drudger, error) {
	return nil, errors.New("a nuke reads no Drudgers")
}

func TestKind(t *testing.T) {
	errBusy := &drudger.DrudgerBusyError{Slot: 2, Sandbox: "drudge-uploader-2", TaskID: testTaskID}
	nukeFeeds := []views.FeedName{views.DrudgersFeed, views.TasksFeed}
	cases := []struct {
		name        string
		service     *fakeDrudgerService
		msg         views.NukeMsg
		wantNukes   []nukeCall
		wantVerdict string
		wantErr     error
		wantHint    string
	}{
		{
			name:        "nuke names the nuked Drudger",
			service:     &fakeDrudgerService{},
			msg:         views.NukeMsg{Slot: 2},
			wantNukes:   []nukeCall{{slot: 2}},
			wantVerdict: "Drudger 2 nuked",
		},
		{
			name:      "refused nuke of a busy Drudger hints at forcing it",
			service:   &fakeDrudgerService{nukeErr: errBusy},
			msg:       views.NukeMsg{Slot: 2, TaskID: testTaskID},
			wantNukes: []nukeCall{{slot: 2}},
			wantErr:   errBusy,
			wantHint:  "busy with 1a2b3c4d, X to force",
		},
		{
			name:        "forced nuke of a busy Drudger goes through",
			service:     &fakeDrudgerService{nukeErr: errBusy},
			msg:         views.NukeMsg{Slot: 2, TaskID: testTaskID, IsForced: true},
			wantNukes:   []nukeCall{{slot: 2, isForced: true}},
			wantVerdict: "Drudger 2 nuked",
		},
		{
			name:      "failed nuke carries the error of the service and no hint",
			service:   &fakeDrudgerService{nukeErr: errDiskOnFire},
			msg:       views.NukeMsg{Slot: 2},
			wantNukes: []nukeCall{{slot: 2}},
			wantErr:   errDiskOnFire,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commands := actions.New(actions.Services{Drudgers: testCase.service}, testProjectSlug, []actions.Kind{nuke.Kind})
			_, cmd := commands.Start(testCase.msg)
			done, ok := cmd().(actions.DoneMsg)
			if !ok {
				t.Fatal("expected a done message")
			}
			if !slices.Equal(testCase.service.nukes, testCase.wantNukes) {
				t.Errorf("expected nukes %v, got %v", testCase.wantNukes, testCase.service.nukes)
			}
			wantCommand := actions.Command{Name: views.NukeCommand, Target: "Drudger 2"}
			if done.Command != wantCommand {
				t.Errorf("expected done command %v, got %v", wantCommand, done.Command)
			}
			if done.Verdict != testCase.wantVerdict {
				t.Errorf("expected verdict %q, got %q", testCase.wantVerdict, done.Verdict)
			}
			if !errors.Is(done.Err, testCase.wantErr) {
				t.Errorf("expected error %v, got %v", testCase.wantErr, done.Err)
			}
			if done.Hint != testCase.wantHint {
				t.Errorf("expected hint %q, got %q", testCase.wantHint, done.Hint)
			}
			if !slices.Equal(done.Feeds, nukeFeeds) {
				t.Errorf("expected feeds %v, got %v", nukeFeeds, done.Feeds)
			}
		})
	}
}

func TestKind_Question(t *testing.T) {
	cases := []struct {
		name         string
		msg          views.NukeMsg
		wantQuestion string
	}{
		{name: "nuke of an idle Drudger", msg: views.NukeMsg{Slot: 2}, wantQuestion: "Nuke Drudger 2? Its sandbox is deleted."},
		{name: "nuke of a busy Drudger", msg: views.NukeMsg{Slot: 2, TaskID: testTaskID}, wantQuestion: "Nuke Drudger 2? Its sandbox is deleted."},
		{name: "forced nuke of an idle Drudger", msg: views.NukeMsg{Slot: 2, IsForced: true}, wantQuestion: "Nuke Drudger 2? Its sandbox is deleted."},
		{name: "forced nuke of a busy Drudger names the task it kills", msg: views.NukeMsg{Slot: 2, TaskID: testTaskID, IsForced: true}, wantQuestion: "Nuke Drudger 2 while it works on 1a2b3c4d? Its agent is killed and the task is fucked up."},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			commands := actions.New(actions.Services{}, testProjectSlug, []actions.Kind{nuke.Kind})
			question, ok := commands.Question(testCase.msg)
			if !ok || question != testCase.wantQuestion {
				t.Errorf("expected question %q, got %q (ok %v)", testCase.wantQuestion, question, ok)
			}
		})
	}
}
