package views

import "github.com/IgorBolotnikov/DRUDGE/internal/task"

// FeedName names a kind of feed.
type FeedName string

// Names of the kinds of feeds. Each one is also the key of its interval in the
// TUI config file.
const (
	TasksFeed        FeedName = "tasks"
	DrudgersFeed     FeedName = "drudgers"
	DrudgerCheckFeed FeedName = "drudgerCheck"
	SessionsFeed     FeedName = "sessions"
)

// Feed names one running feed. TaskID is empty for a feed of the whole project.
type Feed struct {
	Name   FeedName
	TaskID task.TaskID
}
