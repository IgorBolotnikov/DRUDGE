package feeds_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/feeds"
	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// testInterval is short enough that a test runs the tick of a feed right away.
const testInterval = time.Millisecond

const (
	firstFeedName  views.FeedName = "first"
	secondFeedName views.FeedName = "second"
)

var (
	firstFeed  = views.Feed{Name: firstFeedName}
	secondFeed = views.Feed{Name: secondFeedName}
	taskFeed   = views.Feed{Name: firstFeedName, TaskID: "3f9a1c2e"}
)

var errDiskOnFire = errors.New("disk on fire")

// fakeFeed is a kind of feed whose run numbered n returns the message and
// the error at index n-1, and the run number as its state.
type fakeFeed struct {
	name     views.FeedName
	messages []tea.Msg
	errs     []error
	// states holds the state each run got, in the order of the runs.
	states []any
}

func (fake *fakeFeed) kind() feeds.Kind {
	return feeds.Kind{
		Name:            fake.name,
		DefaultInterval: testInterval,
		Run: func(sources feeds.Sources, projectSlug string, feed views.Feed, state any) (tea.Msg, any, error) {
			fake.states = append(fake.states, state)
			runNumber := len(fake.states)
			return at(fake.messages, runNumber), runNumber, at(fake.errs, runNumber)
		},
	}
}

// at returns the value for the run numbered runNumber, or the zero value past
// the end of values.
func at[T any](values []T, runNumber int) T {
	var zero T
	index := runNumber - 1
	if index >= len(values) {
		return zero
	}
	return values[index]
}

func newTestRunner(fakes ...*fakeFeed) *feeds.Runner {
	kinds := make([]feeds.Kind, 0, len(fakes))
	for _, fake := range fakes {
		kinds = append(kinds, fake.kind())
	}
	return feeds.NewRunner(feeds.Sources{}, "uploader", kinds, nil)
}

// runAll runs a command and every command of its batches, and returns their
// messages in order.
func runAll(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	msg := cmd()
	batch, ok := msg.(tea.BatchMsg)
	if !ok {
		return []tea.Msg{msg}
	}
	var messages []tea.Msg
	for _, batched := range batch {
		messages = append(messages, runAll(batched)...)
	}
	return messages
}

// feedMessages splits the messages of commands of a Runner.
type feedMessages struct {
	ticks []feeds.TickMsg
	dones []feeds.DoneMsg
}

func splitMessages(messages []tea.Msg) feedMessages {
	var split feedMessages
	for _, msg := range messages {
		switch msg := msg.(type) {
		case feeds.TickMsg:
			split.ticks = append(split.ticks, msg)
		case feeds.DoneMsg:
			split.dones = append(split.dones, msg)
		}
	}
	return split
}

// syncAndFinish syncs the Runner with wanted, ends every run it starts and
// returns the ticks it armed and the messages for the Views.
func syncAndFinish(runner *feeds.Runner, wanted ...views.Feed) ([]feeds.TickMsg, []tea.Msg) {
	split := splitMessages(runAll(runner.Sync(wanted)))
	return split.ticks, finish(runner, split.dones)
}

// finish ends runs and returns the messages for the Views.
func finish(runner *feeds.Runner, dones []feeds.DoneMsg) []tea.Msg {
	var viewMessages []tea.Msg
	for _, done := range dones {
		if _, viewMsg, _ := runner.HandleDone(done); viewMsg != nil {
			viewMessages = append(viewMessages, viewMsg)
		}
	}
	return viewMessages
}

func TestRunner_Sync(t *testing.T) {
	cases := []struct {
		name string
		// syncs are the wanted feeds of each sync in order.
		syncs      [][]views.Feed
		wantFirst  int
		wantSecond int
	}{
		{name: "a wanted feed runs right away", syncs: [][]views.Feed{{firstFeed}}, wantFirst: 1},
		{name: "a running feed starts no second run", syncs: [][]views.Feed{{firstFeed}, {firstFeed}}, wantFirst: 1},
		{name: "a feed wanted twice runs once", syncs: [][]views.Feed{{firstFeed, firstFeed}}, wantFirst: 1},
		{name: "a feed that stops and starts again runs again", syncs: [][]views.Feed{{firstFeed}, nil, {firstFeed}}, wantFirst: 2},
		{name: "feeds of two kinds both run", syncs: [][]views.Feed{{firstFeed, secondFeed}}, wantFirst: 1, wantSecond: 1},
		{name: "a feed of a task runs apart from the feed of the project", syncs: [][]views.Feed{{firstFeed, taskFeed}}, wantFirst: 2},
		{name: "a feed that is not wanted does not run", syncs: [][]views.Feed{{secondFeed}}, wantSecond: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			first := &fakeFeed{name: firstFeedName}
			second := &fakeFeed{name: secondFeedName}
			runner := newTestRunner(first, second)
			for _, wanted := range testCase.syncs {
				syncAndFinish(runner, wanted...)
			}
			if got := len(first.states); got != testCase.wantFirst {
				t.Errorf("expected %d runs of the first feed, got %d", testCase.wantFirst, got)
			}
			if got := len(second.states); got != testCase.wantSecond {
				t.Errorf("expected %d runs of the second feed, got %d", testCase.wantSecond, got)
			}
		})
	}
}

func TestRunner_SyncArmsOneTick(t *testing.T) {
	runner := newTestRunner(&fakeFeed{name: firstFeedName})
	ticks, _ := syncAndFinish(runner, firstFeed)
	if len(ticks) != 1 {
		t.Fatalf("expected a feed that starts to arm one tick, got %d", len(ticks))
	}
	if ticks, _ := syncAndFinish(runner, firstFeed); len(ticks) != 0 {
		t.Errorf("expected a running feed to arm no other tick, got %d", len(ticks))
	}
}

func TestRunner_HandleTick(t *testing.T) {
	cases := []struct {
		name string
		// tick returns the tick to handle after the first sync, which armed
		// firstTick and left its run going.
		tick      func(runner *feeds.Runner, firstTick feeds.TickMsg, firstRun feeds.DoneMsg) feeds.TickMsg
		wantRuns  int
		wantTicks int
	}{
		{
			name: "a tick runs the feed and arms the next tick",
			tick: func(runner *feeds.Runner, firstTick feeds.TickMsg, firstRun feeds.DoneMsg) feeds.TickMsg {
				runner.HandleDone(firstRun)
				return firstTick
			},
			wantRuns:  2,
			wantTicks: 1,
		},
		{
			name: "a tick while a run is going starts no second run",
			tick: func(runner *feeds.Runner, firstTick feeds.TickMsg, firstRun feeds.DoneMsg) feeds.TickMsg {
				return firstTick
			},
			wantRuns:  1,
			wantTicks: 1,
		},
		{
			name: "a tick of a stopped feed starts nothing",
			tick: func(runner *feeds.Runner, firstTick feeds.TickMsg, firstRun feeds.DoneMsg) feeds.TickMsg {
				runner.HandleDone(firstRun)
				runner.Sync(nil)
				return firstTick
			},
			wantRuns: 1,
		},
		{
			name: "the old tick of a feed started again starts nothing",
			tick: func(runner *feeds.Runner, firstTick feeds.TickMsg, firstRun feeds.DoneMsg) feeds.TickMsg {
				runner.HandleDone(firstRun)
				runner.Sync(nil)
				syncAndFinish(runner, firstFeed)
				return firstTick
			},
			wantRuns: 2,
		},
		{
			name: "the new tick of a feed started again runs it",
			tick: func(runner *feeds.Runner, firstTick feeds.TickMsg, firstRun feeds.DoneMsg) feeds.TickMsg {
				runner.HandleDone(firstRun)
				runner.Sync(nil)
				ticks, _ := syncAndFinish(runner, firstFeed)
				return ticks[0]
			},
			wantRuns:  3,
			wantTicks: 1,
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := &fakeFeed{name: firstFeedName}
			runner := newTestRunner(fake)
			started := splitMessages(runAll(runner.Sync([]views.Feed{firstFeed})))
			tick := testCase.tick(runner, started.ticks[0], started.dones[0])

			handled := splitMessages(runAll(runner.HandleTick(tick)))

			if got := len(fake.states); got != testCase.wantRuns {
				t.Errorf("expected %d runs, got %d", testCase.wantRuns, got)
			}
			if got := len(handled.ticks); got != testCase.wantTicks {
				t.Errorf("expected %d ticks armed, got %d", testCase.wantTicks, got)
			}
		})
	}
}

func TestRunner_Refresh(t *testing.T) {
	cases := []struct {
		name  string
		names []views.FeedName
		// isFirstRunDone is whether the first run of the feed ends before the
		// refresh.
		isFirstRunDone bool
		wantRuns       int
	}{
		{name: "a refreshed feed runs right away", names: []views.FeedName{firstFeedName}, isFirstRunDone: true, wantRuns: 2},
		{name: "a feed of another name does not run", names: []views.FeedName{secondFeedName}, isFirstRunDone: true, wantRuns: 1},
		{name: "a refreshed feed whose run is going runs again when it ends", names: []views.FeedName{firstFeedName}, wantRuns: 2},
		{name: "a feed whose run is going and is not refreshed runs once", names: []views.FeedName{secondFeedName}, wantRuns: 1},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := &fakeFeed{name: firstFeedName}
			runner := newTestRunner(fake, &fakeFeed{name: secondFeedName})
			started := splitMessages(runAll(runner.Sync([]views.Feed{firstFeed})))
			if testCase.isFirstRunDone {
				runner.HandleDone(started.dones[0])
			}

			refreshed := splitMessages(runAll(runner.Refresh(testCase.names)))
			if !testCase.isFirstRunDone {
				_, _, rerun := runner.HandleDone(started.dones[0])
				refreshed = splitMessages(runAll(rerun))
			}

			if got := len(fake.states); got != testCase.wantRuns {
				t.Errorf("expected %d runs, got %d", testCase.wantRuns, got)
			}
			if got := len(refreshed.ticks); got != 0 {
				t.Errorf("expected a refresh to arm no tick, got %d", got)
			}
		})
	}
}

func TestRunner_HandleDone(t *testing.T) {
	cases := []struct {
		name     string
		messages []tea.Msg
		errs     []error
		// runCount is how many runs end, the first by the sync and the rest
		// by ticks.
		runCount         int
		wantViewMessages []tea.Msg
		wantLast         []tea.Msg
		wantErrs         []error
	}{
		{name: "a message reaches the views", messages: []tea.Msg{"tasks"}, runCount: 1, wantViewMessages: []tea.Msg{"tasks"}, wantLast: []tea.Msg{"tasks"}},
		{name: "a nil message reaches no view", runCount: 1},
		{
			name:             "a failed run keeps the last message",
			messages:         []tea.Msg{"tasks", "no tasks"},
			errs:             []error{nil, errDiskOnFire},
			runCount:         2,
			wantViewMessages: []tea.Msg{"tasks", "no tasks"},
			wantLast:         []tea.Msg{"tasks"},
			wantErrs:         []error{errDiskOnFire},
		},
		{
			name:             "a run that works clears the error",
			messages:         []tea.Msg{"no tasks", "tasks"},
			errs:             []error{errDiskOnFire},
			runCount:         2,
			wantViewMessages: []tea.Msg{"no tasks", "tasks"},
			wantLast:         []tea.Msg{"tasks"},
		},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := &fakeFeed{name: firstFeedName, messages: testCase.messages, errs: testCase.errs}
			runner := newTestRunner(fake)
			ticks, viewMessages := syncAndFinish(runner, firstFeed)
			for range testCase.runCount - 1 {
				handled := splitMessages(runAll(runner.HandleTick(ticks[0])))
				ticks = handled.ticks
				viewMessages = append(viewMessages, finish(runner, handled.dones)...)
			}

			if !slices.Equal(viewMessages, testCase.wantViewMessages) {
				t.Errorf("expected the views to get %v, got %v", testCase.wantViewMessages, viewMessages)
			}
			if got := runner.LastMessages([]views.Feed{firstFeed}); !slices.Equal(got, testCase.wantLast) {
				t.Errorf("expected the last messages %v, got %v", testCase.wantLast, got)
			}
			if got := runner.Errors(); !slices.Equal(got, testCase.wantErrs) {
				t.Errorf("expected the errors %v, got %v", testCase.wantErrs, got)
			}
		})
	}
}

func TestRunner_State(t *testing.T) {
	cases := []struct {
		name string
		// shouldRestart stops and starts the feed between its two runs.
		shouldRestart bool
		wantStates    []any
	}{
		{name: "the state of a run reaches the next run", wantStates: []any{nil, 1}},
		{name: "a feed that starts again starts from an empty state", shouldRestart: true, wantStates: []any{nil, nil}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := &fakeFeed{name: firstFeedName}
			runner := newTestRunner(fake)
			ticks, _ := syncAndFinish(runner, firstFeed)
			if testCase.shouldRestart {
				runner.Sync(nil)
				syncAndFinish(runner, firstFeed)
			} else {
				finish(runner, splitMessages(runAll(runner.HandleTick(ticks[0]))).dones)
			}
			if !slices.Equal(fake.states, testCase.wantStates) {
				t.Errorf("expected the runs to get the states %v, got %v", testCase.wantStates, fake.states)
			}
		})
	}
}

func TestRunner_StartsWithLastMessage(t *testing.T) {
	cases := []struct {
		name string
		feed views.Feed
		// isRunFirst ends the fresh run before the last message arrives.
		isRunFirst       bool
		wantViewMessages []tea.Msg
		wantLast         []tea.Msg
	}{
		{name: "a feed of the project sends its last message first", feed: firstFeed, wantViewMessages: []tea.Msg{"old", "new"}, wantLast: []tea.Msg{"new"}},
		{name: "a last message older than the fresh run reaches no view", feed: firstFeed, isRunFirst: true, wantViewMessages: []tea.Msg{"new"}, wantLast: []tea.Msg{"new"}},
		{name: "a feed of a task drops its last message when it stops", feed: taskFeed, wantViewMessages: []tea.Msg{"new"}, wantLast: []tea.Msg{"new"}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			fake := &fakeFeed{name: firstFeedName, messages: []tea.Msg{"old", "new"}}
			runner := newTestRunner(fake)
			syncAndFinish(runner, testCase.feed)
			runner.Sync(nil)

			dones := splitMessages(runAll(runner.Sync([]views.Feed{testCase.feed}))).dones
			if testCase.isRunFirst {
				slices.Reverse(dones)
			}
			viewMessages := finish(runner, dones)

			if !slices.Equal(viewMessages, testCase.wantViewMessages) {
				t.Errorf("expected the views to get %v, got %v", testCase.wantViewMessages, viewMessages)
			}
			if got := runner.LastMessages([]views.Feed{testCase.feed}); !slices.Equal(got, testCase.wantLast) {
				t.Errorf("expected the last messages %v, got %v", testCase.wantLast, got)
			}
		})
	}
}

func TestRunner_DropsTheRunOfAStoppedFeed(t *testing.T) {
	runner := newTestRunner(&fakeFeed{name: firstFeedName, messages: []tea.Msg{"tasks"}, errs: []error{errDiskOnFire}})
	dones := splitMessages(runAll(runner.Sync([]views.Feed{firstFeed}))).dones
	runner.Sync(nil)

	if viewMessages := finish(runner, dones); len(viewMessages) != 0 {
		t.Errorf("expected the run of a stopped feed to reach no view, got %v", viewMessages)
	}
	if errs := runner.Errors(); len(errs) != 0 {
		t.Errorf("expected no error of a stopped feed, got %v", errs)
	}
}

func TestRunner_Errors(t *testing.T) {
	errFirst := errors.New("first failed")
	errSecond := errors.New("second failed")
	cases := []struct {
		name   string
		wanted []views.Feed
		// stillWanted are the feeds still wanted after the runs end.
		stillWanted []views.Feed
		wantErrs    []error
	}{
		{name: "errors follow the order of the kinds", wanted: []views.Feed{secondFeed, firstFeed}, stillWanted: []views.Feed{secondFeed, firstFeed}, wantErrs: []error{errFirst, errSecond}},
		{name: "a feed that stops takes its error with it", wanted: []views.Feed{secondFeed, firstFeed}, stillWanted: []views.Feed{secondFeed}, wantErrs: []error{errSecond}},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			runner := newTestRunner(
				&fakeFeed{name: firstFeedName, errs: []error{errFirst}},
				&fakeFeed{name: secondFeedName, errs: []error{errSecond}},
			)
			syncAndFinish(runner, testCase.wanted...)
			runner.Sync(testCase.stillWanted)
			if got := runner.Errors(); !slices.Equal(got, testCase.wantErrs) {
				t.Errorf("expected the errors %v, got %v", testCase.wantErrs, got)
			}
		})
	}
}
