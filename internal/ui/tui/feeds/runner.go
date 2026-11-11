package feeds

import (
	"cmp"
	"fmt"
	"slices"
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/IgorBolotnikov/DRUDGE/internal/ui/tui/views"
)

// TickMsg starts a run of a feed when its interval is up. It carries the
// generation of the feed it was armed for.
type TickMsg struct {
	feed       views.Feed
	generation int
}

// DoneMsg carries the end of a run of a feed to the Runner, or the last
// message of a feed that started again.
type DoneMsg struct {
	feed       views.Feed
	generation int
	msg        tea.Msg
	next       any
	err        error
	isReplay   bool
}

// runningFeed is what the Runner keeps of one running feed.
type runningFeed struct {
	kind      Kind
	isRunning bool
	// hasRun is whether a run of this generation ended. The last message of
	// an earlier generation is older than any such run.
	hasRun     bool
	err        error
	state      any
	generation int
}

// Runner runs the feeds the Views want. Each running feed runs right away and
// then on every tick of its interval. A tick never starts a second run of a
// feed while one is going.
type Runner struct {
	sources     Sources
	projectSlug string
	kinds       []Kind
	intervals   map[views.FeedName]time.Duration
	running     map[views.Feed]*runningFeed
	// lastMessages holds the message of the last run that worked of each
	// feed. A stopped feed of the whole project keeps it.
	lastMessages   map[views.Feed]tea.Msg
	lastGeneration int
}

// NewRunner returns a Runner of the feeds of a project with no feed running.
// A feed with no interval in intervals runs at the default interval of its
// kind.
func NewRunner(sources Sources, projectSlug string, kinds []Kind, intervals map[views.FeedName]time.Duration) *Runner {
	return &Runner{
		sources:      sources,
		projectSlug:  projectSlug,
		kinds:        kinds,
		intervals:    intervals,
		running:      map[views.Feed]*runningFeed{},
		lastMessages: map[views.Feed]tea.Msg{},
	}
}

// Sync starts every wanted feed that does not run yet and stops every running
// feed that is not wanted. It returns the commands of the feeds it started:
// the last message of the feed when it has one, a run and the tick. It panics
// on a feed of a kind the Runner does not know, which is a bug in the View
// that asked for it.
func (runner *Runner) Sync(wanted []views.Feed) tea.Cmd {
	for feed := range runner.running {
		if !slices.Contains(wanted, feed) {
			runner.stop(feed)
		}
	}
	var commands []tea.Cmd
	for _, feed := range wanted {
		if _, isStarted := runner.running[feed]; !isStarted {
			commands = append(commands, runner.start(feed))
		}
	}
	return tea.Batch(commands...)
}

func (runner *Runner) start(feed views.Feed) tea.Cmd {
	kind, ok := runner.findKind(feed.Name)
	if !ok {
		panic(fmt.Sprintf("a view asked for feed %q, which no feed kind runs", feed.Name))
	}
	runner.lastGeneration++
	started := &runningFeed{kind: kind, generation: runner.lastGeneration}
	runner.running[feed] = started
	var replay tea.Cmd
	if lastMsg, ok := runner.lastMessages[feed]; ok {
		replayed := DoneMsg{feed: feed, generation: started.generation, msg: lastMsg, isReplay: true}
		replay = func() tea.Msg { return replayed }
	}
	return tea.Batch(replay, runner.run(feed, started), runner.tick(feed, started))
}

// stop drops a running feed with its error and its state. A feed of one task
// also drops its last message.
func (runner *Runner) stop(feed views.Feed) {
	delete(runner.running, feed)
	if feed.TaskID != "" {
		delete(runner.lastMessages, feed)
	}
}

// run returns the command of one run of a feed.
func (runner *Runner) run(feed views.Feed, running *runningFeed) tea.Cmd {
	running.isRunning = true
	sources, projectSlug, state, generation := runner.sources, runner.projectSlug, running.state, running.generation
	runFeed := running.kind.Run
	return func() tea.Msg {
		msg, next, err := runFeed(sources, projectSlug, feed, state)
		return DoneMsg{feed: feed, generation: generation, msg: msg, next: next, err: err}
	}
}

// tick returns the command of the next tick of a feed.
func (runner *Runner) tick(feed views.Feed, running *runningFeed) tea.Cmd {
	generation := running.generation
	return tea.Tick(runner.interval(running.kind), func(time.Time) tea.Msg {
		return TickMsg{feed: feed, generation: generation}
	})
}

func (runner *Runner) interval(kind Kind) time.Duration {
	if interval, ok := runner.intervals[kind.Name]; ok {
		return interval
	}
	return kind.DefaultInterval
}

// HandleTick starts a run of the feed of a tick and arms its next tick. It
// skips the run while the last one is going. It drops a tick of a feed that
// stopped since the tick was armed.
func (runner *Runner) HandleTick(msg TickMsg) tea.Cmd {
	running, ok := runner.current(msg.feed, msg.generation)
	if !ok {
		return nil
	}
	nextTick := runner.tick(msg.feed, running)
	if running.isRunning {
		return nextTick
	}
	return tea.Batch(runner.run(msg.feed, running), nextTick)
}

// HandleDone ends a run of a feed. It keeps the error and the state of the
// run, and the message of a run that worked. It returns the feed and the
// message for the Views of the feed. The message is nil for a run of a feed
// that stopped since, and for the last message of a feed that has run again
// since.
func (runner *Runner) HandleDone(msg DoneMsg) (views.Feed, tea.Msg) {
	running, ok := runner.current(msg.feed, msg.generation)
	if !ok {
		return msg.feed, nil
	}
	if msg.isReplay {
		if running.hasRun {
			return msg.feed, nil
		}
		return msg.feed, msg.msg
	}
	running.isRunning = false
	running.hasRun = true
	running.err = msg.err
	running.state = msg.next
	if msg.err == nil && msg.msg != nil {
		runner.lastMessages[msg.feed] = msg.msg
	}
	return msg.feed, msg.msg
}

// current returns the running feed a message of a generation belongs to. ok
// is false when the feed stopped since, and started again or not.
func (runner *Runner) current(feed views.Feed, generation int) (running *runningFeed, ok bool) {
	running, ok = runner.running[feed]
	if !ok || running.generation != generation {
		return nil, false
	}
	return running, true
}

// LastMessages returns the message of the last run that worked of each feed
// that has one, in the order of the feeds.
func (runner *Runner) LastMessages(feeds []views.Feed) []tea.Msg {
	var messages []tea.Msg
	for _, feed := range feeds {
		if lastMsg, ok := runner.lastMessages[feed]; ok {
			messages = append(messages, lastMsg)
		}
	}
	return messages
}

// Errors returns the error of the last run of every running feed whose last
// run failed, in the order of the kinds and then of the task IDs.
func (runner *Runner) Errors() []error {
	var failed []views.Feed
	for feed, running := range runner.running {
		if running.err != nil {
			failed = append(failed, feed)
		}
	}
	slices.SortFunc(failed, func(first, second views.Feed) int {
		return cmp.Or(
			cmp.Compare(runner.kindIndex(first.Name), runner.kindIndex(second.Name)),
			cmp.Compare(first.TaskID, second.TaskID),
		)
	})
	errs := make([]error, 0, len(failed))
	for _, feed := range failed {
		errs = append(errs, runner.running[feed].err)
	}
	return errs
}

func (runner *Runner) findKind(name views.FeedName) (Kind, bool) {
	index := runner.kindIndex(name)
	if index == -1 {
		return Kind{}, false
	}
	return runner.kinds[index], true
}

func (runner *Runner) kindIndex(name views.FeedName) int {
	return slices.IndexFunc(runner.kinds, func(kind Kind) bool { return kind.Name == name })
}
